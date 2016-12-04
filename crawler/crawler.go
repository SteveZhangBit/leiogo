package crawler

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/middleware"
	"github.com/SteveZhangBit/log"
)

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
}

func (c *Crawler) addRequest(req *leiogo.Request) {
	c.count.Add()
	go func() { c.requests <- req }()
}

func (c *Crawler) Crawl(spider *leiogo.Spider) {
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
		m.Close("Jobs completed", spider)
	}
	for _, m := range c.SpiderMiddlewares {
		m.Close("Jobs completed", spider)
	}
	for _, m := range c.ItemPipelines {
		m.Close("Jobs completed", spider)
	}
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
}

func (c *Crawler) NewRequest(req *leiogo.Request, parRes *leiogo.Response, spider *leiogo.Spider) {
	if parRes != nil {
		for _, m := range c.SpiderMiddlewares {
			if ok := c.handleErr(m.ProcessNewRequest(req, parRes, spider), req, m, spider); !ok {
				return
			}
		}
	}
	c.addRequest(req)
}

func (c *Crawler) NewItem(item *leiogo.Item, spider *leiogo.Spider) {
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
}
