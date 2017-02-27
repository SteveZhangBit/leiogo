package crawler

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/log"
	"github.com/SteveZhangBit/leiogo/middleware"
)

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

	// In some case, we want to add some additional spider open/close listeners which do
	// not belong to any middleware, usually they only implement the OpenClose interface.
	OpenCloses []middleware.OpenClose

	// There should be at least one parser named 'default'.
	Parsers map[string]middleware.Parser

	ItemPipelines []middleware.ItemPipeline

	// StatusInfo contains the basic information about this crawler,
	// and the crawler will print this information when it stops.
	// More details can be found in the struct defination.
	StatusInfo StatusInfo
}

func (c *Crawler) addRequest(req *leiogo.Request) {
	// Add a new request to the queue. Pay attention that we call the channel method
	// in a new goroutine, in case deadlock problem.
	if !c.StatusInfo.IsInterrupt() {
		c.StatusInfo.AddPage()
		c.count.Add()
		go func() { c.requests <- req }()
	}
}

// After finishing initializing the crawler, call this method to start the spider.
func (c *Crawler) Crawl(spider *leiogo.Spider) {
	c.Logger.Info(spider.Name, "Start spider")
	// When starting the spider, we have to call all the Open methods of the middlewares.
	// TODO: These lines should be refined in the future.
	for _, m := range c.OpenCloses {
		m.Open(spider)
	}
	for _, m := range c.DownloadMiddlewares {
		m.Open(spider)
	}
	for _, m := range c.SpiderMiddlewares {
		m.Open(spider)
	}
	for _, m := range c.ItemPipelines {
		m.Open(spider)
	}

	// If there isn't any start urls, then directly close the spider.
	// Otherwise, the program will wait forever.
	if len(spider.StartURLs) != 0 {

		// Wait for all the requests to complete.
		// This should be invoked before any addRequest,
		// otherwise the program will block forever.
		go func() {
			c.count.Wait()
			close(c.requests)
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
	}

	c.Logger.Info(spider.Name, "Closing spider")
	// TODO: These lines are the same to the Open methods above and should be refined in the future.
	for _, m := range c.ItemPipelines {
		m.Close(c.StatusInfo.Reason, spider)
	}
	for _, m := range c.SpiderMiddlewares {
		m.Close(c.StatusInfo.Reason, spider)
	}
	for _, m := range c.DownloadMiddlewares {
		m.Close(c.StatusInfo.Reason, spider)
	}
	for _, m := range c.OpenCloses {
		m.Close(c.StatusInfo.Reason, spider)
	}
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
	c.StatusInfo.AddRunningPage(req)

	for _, m := range c.DownloadMiddlewares {
		if ok := c.handleErr(m.ProcessRequest(req, spider), req, m, spider); !ok {
			return
		}
	}

	res := c.Downloader.Download(req, spider)
	c.StatusInfo.AddCrawled()

	// Check whether the request is a static file request.
	if typeName, ok := req.Meta["__type__"]; ok && typeName.(string) == "file" {

		// In order to get the right count, we have the make sure that the
		// the response shows that the download is completed, which means
		// a DropTaskErr in the Err field.
		switch res.Err.(type) {
		case *middleware.DropTaskError:
			c.StatusInfo.AddFiles()
		default:
		}
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

	if parser, ok := c.Parsers[req.ParserName]; !ok {
		c.Logger.Error(spider.Name, "No parser named %s", req.ParserName)
	} else {
		parser(res, req, spider)
	}
	c.StatusInfo.AddSucceed(req)
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
	c.StatusInfo.AddItem()
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
