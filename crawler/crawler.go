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

	// Number of pages yielded by the spider, including the droped ones.
	Pages int

	// Number of pages successfully downloaded by the downloader.
	Crawled int

	// Number of pages successfully reaching the parsers.
	Succeed int

	// All items yielded, including the droped ones.
	Items int

	// If user enable image download feature for the crawler, this field will show how many images have downloaded.
	Images int

	Reason string
	Closed bool
}

type Crawler struct {
	// The buffered channel object for producing and consuming requests.
	requests chan *leiogo.Request

	// Tokens are used to controll the concurrent requests at the same time.
	// See ConcurrentRequests in context.go for more information.
	tokens chan struct{}

	// This is similar to os/signal workgroup, in order to make the crawler to wait
	// for all the requests to complete.
	count ConcurrentCount

	Logger              log.Logger
	DownloadMiddlewares []middleware.DownloadMiddleware
	SpiderMiddlewares   []middleware.SpiderMiddleware
	Downloader          middleware.Downloader

	// There should be at least one parser named 'default'.
	Parsers map[string]middleware.Parser

	ItemPipelines []middleware.ItemPipeline

	// StatusInfo contains the basic information about this crawler,
	// and the crawler will print this information when it stops.
	// More details can be found in the struct defination.
	StatusInfo StatusInfo

	// Add support for phantomjs. If user add 'phantomjs' = true to the requests' meta,
	// and set phantomjsSupport to true when creating the crawler, such requests will
	// be processed by phantomjs in a subprocess.
	Phantomjs middleware.Downloader
}

func (c *Crawler) addRequest(req *leiogo.Request) {
	// Add a new request to the queue. Pay attention that we call the channel method
	// in a new goroutine, in case deadlock problem.
	if !c.StatusInfo.Closed {
		c.StatusInfo.Pages++
		c.count.Add()
		go func() { c.requests <- req }()
	}
}

// After finishing initializing the crawler, call this method to start the spider.
func (c *Crawler) Crawl(spider *leiogo.Spider) {
	c.StatusInfo.StartDate = time.Now()
	c.StatusInfo.Reason = "Jobs completed"

	c.Logger.Info(spider.Name, "Start spider")
	// When starting the spider, we have to call all the Open methods of the middlewares.
	// TODO: These lines should be refined in the future.
	for _, m := range c.DownloadMiddlewares {
		m.Open(spider)
	}
	for _, m := range c.SpiderMiddlewares {
		m.Open(spider)
	}
	for _, m := range c.ItemPipelines {
		m.Open(spider)
	}

	// Wait for all the requests to complete.
	go func() {
		c.count.Wait()
		close(c.requests)
	}()

	// The crawler will catch the interrupt signal from OS.
	// The process won't stop immediately when user press ctrl+c, instead,
	// it will wait for the running requests and items to complete,
	// and refuse any further product.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		c.StatusInfo.Closed = true
		c.StatusInfo.Reason = "User interrupted"
		c.Logger.Info(spider.Name, "Get user interrupt signal, waiting the running requests to complete")
	}()

	c.Logger.Info(spider.Name, "Adding start URLs")
	for _, req := range spider.StartURLs {
		c.addRequest(req)
	}

	for req := range c.requests {
		// In order to controll the concurrent requests, we use a special channel.
		// To process a new request, we should first get a token. If there's no token remaining,
		// the thread will wait.
		c.tokens <- struct{}{}
		go func(_req *leiogo.Request) {
			c.crawl(_req, spider)
			c.count.Done()

			// After a request has completed, release a token.
			<-c.tokens
		}(req)
	}

	c.Logger.Info(spider.Name, "Closing spider")
	// TODO: These lines are the same to the Open methods above and should be refined in the future.
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
	c.Logger.Info(spider.Name, "%-10s - %d", "Images", c.StatusInfo.Images)
	c.Logger.Info(spider.Name, "%-10s - %s", "Reason", c.StatusInfo.Reason)
}

// When there's a error from the middleware, first we need to identify whether it's a DropTaskError.
// And for other error, we just call the HandleErr method in each middleware. Users are able to override
// the method.
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

// This is the main method of crawler. Every request, after passing through the processNewRequest method
// in spider middleware, it wil start its journey here: processRequest in download middleware ->
// downlader -> processResponse in download middleware -> processResponse in spider middleware ->
// user defined parser (by ParserName in request).
// PS: these's a exception here, all the new requests in startURLs will not pass through the processNewRequest method
// in spider middleware. This is a technical design :)
// See more information about middlewares in middleware package.
func (c *Crawler) crawl(req *leiogo.Request, spider *leiogo.Spider) {
	for _, m := range c.DownloadMiddlewares {
		if ok := c.handleErr(m.ProcessRequest(req, spider), req, m, spider); !ok {
			return
		}
	}

	// We add a new feature here, phantomjs. Since we still need our default downloader to
	// download those static files like images, so we make them to be two different downloaders.
	// When a request is taged with 'phantomjs' = true as well as the phantomjs downloader is not nil,
	// we should use phantomjs to dowload this request to get javascript runtime to support AJAX.
	var res *leiogo.Response
	if enable, ok := req.Meta["phantomjs"]; ok && enable.(bool) && c.Phantomjs != nil {
		res = c.Phantomjs.Download(req, spider)
	} else {
		res = c.Downloader.Download(req, spider)
	}
	c.StatusInfo.Crawled++

	// Check whether the request is a static file request.
	if typeName, ok := req.Meta["type"]; ok && typeName.(string) == "file" {
		c.StatusInfo.Images++
	}

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

// Create a new request, pay attention that we have to pass in the parent response here.
// Eevry request will first pass through the processNewRequest method here.
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

// Create a new item, and make it pass through the item pipelines.
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
