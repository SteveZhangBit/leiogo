package crawler

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/middleware"
	"github.com/SteveZhangBit/log"
)

type CrawlerBuilder struct {
	Crawler *Crawler
}

func (c *CrawlerBuilder) Build() *Crawler {
	return c.Crawler
}

func CreateCrawlerBuilder() *CrawlerBuilder {
	return &CrawlerBuilder{Crawler: &Crawler{
		requests:            make(chan *leiogo.Request, 1),
		tokens:              make(chan struct{}, ConcurrentRequests),
		count:               ConcurrentCount{done: make(chan bool, 1)},
		Logger:              log.New("Crawler"),
		DownloadMiddlewares: make([]middleware.DownloadMiddleware, 0),
		SpiderMiddlewares:   make([]middleware.SpiderMiddleware, 0),
		Parsers:             make(map[string]middleware.Parser),
		ItemPipelines:       make([]middleware.ItemPipeline, 0),
		Downloader:          NewDownloader(),
	}}
}

func DefaultCrawlerBuilder() *CrawlerBuilder {
	c := CreateCrawlerBuilder()
	c.AddDownloadMiddlewares(
		NewOffSiteMiddleware(),
		NewDelayMiddleware(),
		NewRetryMiddleware(c.Crawler),
		NewCacheMiddleware(),
	)
	c.AddSpiderMiddlewares(
		NewHttpErrorMiddleware(),
		NewDepthMiddleware(),
	)
	return c
}

func (c *CrawlerBuilder) AddDownloadMiddlewares(ms ...middleware.DownloadMiddleware) *CrawlerBuilder {
	for _, m := range ms {
		c.Crawler.DownloadMiddlewares = append(c.Crawler.DownloadMiddlewares, m)
	}
	return c
}

func (c *CrawlerBuilder) AddSpiderMiddlewares(ms ...middleware.SpiderMiddleware) *CrawlerBuilder {
	for _, m := range ms {
		c.Crawler.SpiderMiddlewares = append(c.Crawler.SpiderMiddlewares, m)
	}
	return c
}

func (c *CrawlerBuilder) SetDownloader(d middleware.Downloader) *CrawlerBuilder {
	c.Crawler.Downloader = d
	return c
}

func (c *CrawlerBuilder) AddPhantomjsSupport() *CrawlerBuilder {
	c.Crawler.Phantomjs = NewPhantomDownloader()
	return c
}

func (c *CrawlerBuilder) AddParser(name string, p middleware.Parser) *CrawlerBuilder {
	c.Crawler.Parsers[name] = p
	return c
}

func (c *CrawlerBuilder) AddItemPipelines(ps ...middleware.ItemPipeline) *CrawlerBuilder {
	for _, p := range ps {
		c.Crawler.ItemPipelines = append(c.Crawler.ItemPipelines, p)
	}
	return c
}

func (c *CrawlerBuilder) AddImageDownloadSupport(path string) *CrawlerBuilder {
	c.AddSpiderMiddlewares(NewSaveImageMiddleware())
	c.AddItemPipelines(NewImagePipeline(path, c.Crawler))
	return c
}
