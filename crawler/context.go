package crawler

import (
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

type DefaultParser struct {
	*Crawler
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
