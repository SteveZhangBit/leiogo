package crawler

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/middleware"
	"github.com/SteveZhangBit/log"
	"os"
	"os/signal"
	"time"
)

type StatusInfo struct {
	StartDate time.Time
	EndDate   time.Time
	Pages     int
	Crawled   int
	Succeed   int
	Items     int
	Reason    string
	Closed    bool
}

type Crawler struct {
	requests            chan *leiogo.Request
	tokens              chan struct{}
	count               ConcurrentCount
	Logger              log.Logger
	DownloadMiddlewares []middleware.DownloadMiddleware
	SpiderMiddlewares   []middleware.SpiderMiddleware
	Downloader          middleware.Downloader
	Parsers             map[string]middleware.Parser
	ItemPipelines       []middleware.ItemPipeline
	StatusInfo          StatusInfo
}

func (c *Crawler) addRequest(req *leiogo.Request) {
	if !c.StatusInfo.Closed {
		c.StatusInfo.Pages++
		c.count.Add()
		go func() { c.requests <- req }()
	}
}

func (c *Crawler) Crawl(spider *leiogo.Spider) {
	c.StatusInfo.StartDate = time.Now()
	c.StatusInfo.Reason = "Jobs completed"

	c.Logger.Info(spider.Name, "Start spider")
	for _, m := range c.DownloadMiddlewares {
		m.Open(spider)
	}
	for _, m := range c.SpiderMiddlewares {
		m.Open(spider)
	}
	for _, m := range c.ItemPipelines {
		m.Open(spider)
	}

	go func() {
		c.count.Wait()
		close(c.requests)
	}()

	interupt := make(chan os.Signal, 1)
	signal.Notify(interupt, os.Interrupt)
	go func() {
		<-interupt
		c.StatusInfo.Closed = true
		c.StatusInfo.Reason = "User interrupted"
		c.Logger.Info(spider.Name, "Get user interrupt signal, waiting the running requests to complete")
	}()

	c.Logger.Info(spider.Name, "Adding start URLs")
	for _, req := range spider.StartURLs {
		c.addRequest(req)
	}

	for req := range c.requests {
		c.tokens <- struct{}{}
		go func(_req *leiogo.Request) {
			c.crawl(_req, spider)
			c.count.Done()

			<-c.tokens
		}(req)
	}

	c.Logger.Info(spider.Name, "Closing spider")
	for _, m := range c.DownloadMiddlewares {
		m.Close(c.StatusInfo.Reason, spider)
	}
	for _, m := range c.SpiderMiddlewares {
		m.Close(c.StatusInfo.Reason, spider)
	}
	for _, m := range c.ItemPipelines {
		m.Close(c.StatusInfo.Reason, spider)
	}
	c.StatusInfo.EndDate = time.Now()
	c.printStatus(spider)
}

func (c *Crawler) printStatus(spider *leiogo.Spider) {
	c.Logger.Info(spider.Name, "%-10s - %s", "Start Date", c.StatusInfo.StartDate.Format("2006-01-02 15:04:05"))
	c.Logger.Info(spider.Name, "%-10s - %s", "End Date", c.StatusInfo.EndDate.Format("2006-01-02 15:04:05"))
	c.Logger.Info(spider.Name, "%-10s - %d", "Pages", c.StatusInfo.Pages)
	c.Logger.Info(spider.Name, "%-10s - %d", "Crawled", c.StatusInfo.Crawled)
	c.Logger.Info(spider.Name, "%-10s - %d", "Succeed", c.StatusInfo.Succeed)
	c.Logger.Info(spider.Name, "%-10s - %d", "Items", c.StatusInfo.Items)
	c.Logger.Info(spider.Name, "%-10s - %s", "Reason", c.StatusInfo.Reason)
}

func (c *Crawler) handleErr(err error, req *leiogo.Request,
	handler middleware.HandleErr, spider *leiogo.Spider) bool {
	if err != nil {
		switch err.(type) {
		case *middleware.DropTaskError:
			c.Logger.Debug(spider.Name, "Drop task %s, %s", req.URL, err.Error())
		default:
			handler.HandleErr(err, spider)
		}
		return false
	}
	return true
}

func (c *Crawler) crawl(req *leiogo.Request, spider *leiogo.Spider) {
	for _, m := range c.DownloadMiddlewares {
		if ok := c.handleErr(m.ProcessRequest(req, spider), req, m, spider); !ok {
			return
		}
	}

	res := c.Downloader.Download(req, spider)
	c.StatusInfo.Crawled++

	for _, m := range c.DownloadMiddlewares {
		if ok := c.handleErr(m.ProcessResponse(res, req, spider), req, m, spider); !ok {
			return
		}
	}

	for _, m := range c.SpiderMiddlewares {
		if ok := c.handleErr(m.ProcessResponse(res, req, spider), req, m, spider); !ok {
			return
		}
	}

	c.Parsers[req.ParserName](res, req, spider)
	c.StatusInfo.Succeed++
}

func (c *Crawler) NewRequest(req *leiogo.Request, parRes *leiogo.Response, spider *leiogo.Spider) error {
	if parRes != nil {
		for _, m := range c.SpiderMiddlewares {
			if ok := c.handleErr(m.ProcessNewRequest(req, parRes, spider), req, m, spider); !ok {
				return nil
			}
		}
	}
	c.addRequest(req)
	return nil
}

func (c *Crawler) NewItem(item *leiogo.Item, spider *leiogo.Spider) error {
	c.StatusInfo.Items++
	c.count.Add()
	go func() {
		for _, p := range c.ItemPipelines {
			if err := p.Process(item, spider); err != nil {
				switch err.(type) {
				case *middleware.DropItemError:
					c.Logger.Debug(spider.Name, "Drop item %s, %s", item.String(), err.Error())
				default:
					p.HandleErr(err, spider)
				}
				break
			}
		}
		c.count.Done()
	}()
	return nil
}
