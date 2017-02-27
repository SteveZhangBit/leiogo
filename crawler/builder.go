package crawler

import (
	"reflect"

	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/log"
	"github.com/SteveZhangBit/leiogo/middleware"
)

type CrawlerBuilder struct {
	Crawler *Crawler
}

func (c *CrawlerBuilder) Build() *Crawler {
	return c.Crawler
}

func CreateCrawlerBuilder() *CrawlerBuilder {
	builder := &CrawlerBuilder{Crawler: &Crawler{
		requests:   make(chan *leiogo.Request, 1),
		tokens:     make(chan struct{}, ConcurrentRequests),
		count:      ConcurrentCount{done: make(chan bool, 1)},
		Logger:     log.New("Crawler"),
		Parsers:    make(map[string]middleware.Parser),
		Downloader: NewDownloader(),
		StatusInfo: StatusInfo{Logger: log.New("Crawler")},
	}}

	builder.AddOpenCloses(
		&UserInterrupt{Logger: log.New("Crawler"), StatusInfo: &builder.Crawler.StatusInfo},
		&builder.Crawler.StatusInfo,
	)

	return builder
}

func DefaultCrawlerBuilder() *CrawlerBuilder {
	c := CreateCrawlerBuilder()
	c.AddDownloadMiddlewares(
		NewOffSiteMiddleware(),
		NewDelayMiddleware(),
		NewRetryMiddleware(),
		NewCacheMiddleware(),
	)
	c.AddSpiderMiddlewares(
		NewHttpErrorMiddleware(),
		NewReferenceURLMiddleware(),
		NewDepthMiddleware(),
	)
	c.AddItemPipelines(NewFilePipeline(FileSaveDir))
	return c
}

func (c *CrawlerBuilder) addYielder(m interface{}) {
	v := reflect.ValueOf(m).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Type.String() == "middleware.Yielder" {
			v.Field(i).Set(reflect.ValueOf(c.Crawler))
		}
	}
}

func (c *CrawlerBuilder) DefaultParser() DefaultParser {
	return DefaultParser{Crawler: c.Crawler}
}

func (c *CrawlerBuilder) AddDownloadMiddlewares(ms ...middleware.DownloadMiddleware) *CrawlerBuilder {
	for _, m := range ms {
		c.addYielder(m)
		c.Crawler.DownloadMiddlewares = append(c.Crawler.DownloadMiddlewares, m)
	}
	return c
}

func (c *CrawlerBuilder) AddSpiderMiddlewares(ms ...middleware.SpiderMiddleware) *CrawlerBuilder {
	for _, m := range ms {
		c.addYielder(m)
		c.Crawler.SpiderMiddlewares = append(c.Crawler.SpiderMiddlewares, m)
	}
	return c
}

func (c *CrawlerBuilder) SetDownloader(d middleware.Downloader) *CrawlerBuilder {
	c.Crawler.Downloader = d
	return c
}

func (c *CrawlerBuilder) AddParser(name string, p middleware.Parser) *CrawlerBuilder {
	c.Crawler.Parsers[name] = p
	return c
}

func (c *CrawlerBuilder) AddItemPipelines(ps ...middleware.ItemPipeline) *CrawlerBuilder {
	for _, p := range ps {
		c.addYielder(p)
		c.Crawler.ItemPipelines = append(c.Crawler.ItemPipelines, p)
	}
	return c
}

func (c *CrawlerBuilder) AddOpenCloses(ms ...middleware.OpenClose) *CrawlerBuilder {
	for _, m := range ms {
		c.Crawler.OpenCloses = append(c.Crawler.OpenCloses, m)
	}
	return c
}
