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
		Base: middleware.Base{Logger: log.New("OffSiteMiddleware")},
	}
}

func NewDelayMiddleware() middleware.DownloadMiddleware {
	return &middleware.DelayMiddleware{
		Base:           middleware.Base{Logger: log.New("DelayMiddleware")},
		DownloadDelay:  DownloadDelay,
		RandomizeDelay: RandomizeDelay,
	}
}

func NewRetryMiddleware(yielder middleware.Yielder) middleware.DownloadMiddleware {
	return &middleware.RetryMiddleware{
		Base:         middleware.Base{Logger: log.New("RetryMiddleware")},
		RetryEnabled: RetryEnabled,
		RetryTimes:   RetryTimes,
		Yielder:      yielder,
	}
}

func NewCacheMiddleware() middleware.DownloadMiddleware {
	return &middleware.CacheMiddleware{
		Base:  middleware.Base{Logger: log.New("CacheMiddleware")},
		Cache: make(map[string]struct{}),
	}
}

func NewHttpErrorMiddleware() middleware.SpiderMiddleware {
	return &middleware.HttpErrorMiddleware{
		Base: middleware.Base{Logger: log.New("HttpErrorMiddleware")},
	}
}

func NewDepthMiddleware() middleware.SpiderMiddleware {
	return &middleware.DepthMiddleware{
		Base:       middleware.Base{Logger: log.New("DepthMiddleware")},
		DepthLimit: DepthLimit,
	}
}

func NewImagePipeline(dir string, yielder middleware.Yielder) middleware.ItemPipeline {
	return &middleware.ImagePipeline{
		Base:    middleware.Base{Logger: log.New("ImagePipeline")},
		DirPath: dir,
		Yielder: yielder,
	}
}

func NewSaveImageMiddleware() middleware.SpiderMiddleware {
	return &middleware.SaveImageMiddleware{
		Base: middleware.Base{Logger: log.New("SaveImageMiddleware")},
	}
}
