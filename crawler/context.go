package crawler

import (
	"github.com/SteveZhangBit/css/selector"
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/middleware"
	"github.com/SteveZhangBit/log"
	"time"
)

var (
	DepthLimit         = 0
	RandomizeDelay     = true
	DownloadDelay      = 2.0
	RetryEnabled       = true
	RetryTimes         = 3
	Timeout            = time.Duration(30)
	ConcurrentRequests = 32
)

type PatternFunc func(el *selector.Elements) interface{}

type DefaultParser struct {
	*Crawler
}

func (d *DefaultParser) RunPattern(patterns map[string]PatternFunc, res *leiogo.Response, spider *leiogo.Spider) {
	doc := selector.Parse(string(res.Body))
	for key, val := range patterns {
		if el := doc.Find(key); el.Err != nil {
			d.Logger.Error(spider.Name, "Error at querying %s, %s", key, el.Err)
		} else {
			switch x := val(el).(type) {
			case *leiogo.Item:
				d.NewItem(x, spider)
			case []*leiogo.Item:
				for _, item := range x {
					d.NewItem(item, spider)
				}
			case *leiogo.Request:
				d.NewRequest(x, res, spider)
			case []*leiogo.Request:
				for _, req := range x {
					d.NewRequest(req, res, spider)
				}
			default:
				d.Logger.Error(spider.Name, "Unknown return type for patter function %T", x)
			}
		}
	}
}

func NewDownloader() middleware.Downloader {
	return &middleware.DefaultDownloader{
		Logger:       log.New("Downloader"),
		ClientConfig: &middleware.DefaultConfig{Timeout: Timeout},
	}
}

func NewProxyDownloader(url string) middleware.Downloader {
	return &middleware.DefaultDownloader{
		Logger: log.New("ProxyDownloader"),
		ClientConfig: &middleware.ProxyConfig{
			Timeout:  Timeout,
			ProxyURL: url},
	}
}

func NewOffSiteMiddleware() middleware.DownloadMiddleware {
	return &middleware.OffSiteMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("OffSiteMiddleware"),
	}
}

func NewDelayMiddleware() middleware.DownloadMiddleware {
	return &middleware.DelayMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("DelayMiddleware"),
		DownloadDelay:  DownloadDelay,
		RandomizeDelay: RandomizeDelay,
	}
}

func NewRetryMiddleware(yielder middleware.Yielder) middleware.DownloadMiddleware {
	return &middleware.RetryMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("RetryMiddleware"),
		RetryEnabled:   RetryEnabled,
		RetryTimes:     RetryTimes,
		Yielder:        yielder,
	}
}

func NewCacheMiddleware() middleware.DownloadMiddleware {
	return &middleware.CacheMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("CacheMiddleware"),
		Cache:          make(map[string]struct{}),
	}
}

func NewHttpErrorMiddleware() middleware.SpiderMiddleware {
	return &middleware.HttpErrorMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("HttpErrorMiddleware"),
	}
}

func NewDepthMiddleware() middleware.SpiderMiddleware {
	return &middleware.DepthMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("DepthMiddleware"),
		DepthLimit:     DepthLimit,
	}
}

func NewImagePipeline(dir string, yielder middleware.Yielder) middleware.ItemPipeline {
	return &middleware.ImagePipeline{
		Base:    middleware.NewBasePipeline("ImagePipeline"),
		DirPath: dir,
		Yielder: yielder,
	}
}

func NewSaveImageMiddleware() middleware.SpiderMiddleware {
	return &middleware.SaveImageMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("SaveImageMiddleware"),
	}
}
