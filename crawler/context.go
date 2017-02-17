package crawler

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo-css/selector"
	"github.com/SteveZhangBit/leiogo/log"
	"github.com/SteveZhangBit/leiogo/middleware"
)

var (
	DepthLimit         = 0
	RandomizeDelay     = true
	DownloadDelay      = 2.0
	RetryEnabled       = true
	RetryTimes         = 3
	Timeout            = 30
	ConcurrentRequests = 32
	UserAgent          = ""
	FileSaveDir        = "./files"

	// When we want to change the default file writer in downloader,
	// we simply change this value.
	DownloaderFileWriter middleware.FileWriter = &middleware.FSWriter{}
)

type PatternFunc func(el *selector.Elements) []interface{}

type DefaultParser struct {
	*Crawler
}

func (d *DefaultParser) RunPattern(patterns map[string]PatternFunc, res *leiogo.Response, spider *leiogo.Spider) {
	doc := selector.Parse(string(res.Body))
	if doc.Err != nil {
		d.Logger.Error(spider.Name, "Error at parsing response body, %s", doc.Err)
		return
	}

	for key, f := range patterns {
		var el *selector.Elements

		// Sometimes, we can define an empty pattern, meaning that it should not do any css selection
		if key != "" {
			if el = doc.Find(key); el.Err != nil {
				d.Logger.Error(spider.Name, "Error at querying %s, %s", key, el.Err)
				continue
			}
		} else {
			el = doc
		}

		for _, val := range f(el) {
			switch x := val.(type) {
			case *leiogo.Item:
				d.NewItem(x, spider)
			case *leiogo.Request:
				d.NewRequest(x, res, spider)
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
		UserAgent:    UserAgent,
		FileWriter:   DownloaderFileWriter,
	}
}

func NewProxyDownloader(url string) middleware.Downloader {
	return &middleware.DefaultDownloader{
		Logger:       log.New("ProxyDownloader"),
		ClientConfig: &middleware.ProxyConfig{Timeout: Timeout, ProxyURL: url},
		UserAgent:    UserAgent,
		FileWriter:   DownloaderFileWriter,
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

func NewRetryMiddleware() middleware.DownloadMiddleware {
	return &middleware.RetryMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("RetryMiddleware"),
		RetryEnabled:   RetryEnabled,
		RetryTimes:     RetryTimes,
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

func NewReferenceURLMiddleware() middleware.SpiderMiddleware {
	return &middleware.ReferenceURLMiddleware{
		BaseMiddleware: middleware.NewBaseMiddleware("ReferenceURLMiddleware"),
	}
}

func NewFilePipeline(dir string) middleware.ItemPipeline {
	return &middleware.FilePipeline{
		Base:       middleware.NewBasePipeline("FilePipeline"),
		DirPath:    dir,
		FileWriter: DownloaderFileWriter,
	}
}

func NewJSONPipeline(name string) middleware.ItemPipeline {
	return &middleware.JSONPipeline{
		Base:     middleware.NewBasePipeline("JSONPipeline"),
		FileName: name,
	}
}
