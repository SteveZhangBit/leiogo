package middleware

import (
	"fmt"
	"github.com/SteveZhangBit/leiogo"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"
)

// When a middleware wants to drop the current task,
// return this type of error
type DropTaskError struct {
	Message string
}

func (err *DropTaskError) Error() string {
	return err.Message
}

// Using CacheMiddleware to store the crawled urls and avoid duplicated urls.
// Cause each middleware will be called in different goroutines,
// So Locking is necessary.
type CacheMiddleware struct {
	BaseMiddleware
	Cache map[string]struct{}
	mutex sync.RWMutex
}

func (m *CacheMiddleware) ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	m.Logger.Debug(spider.Name, "Test whether %s is cached", req.URL)
	if _, ok := m.Cache[req.URL]; ok {
		return &DropTaskError{Message: "URL already parsed"}
	}
	return nil
}

func (m *CacheMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.Logger.Debug(spider.Name, "Add %s to cache", req.URL)
	m.Cache[req.URL] = struct{}{}
	return nil
}

// Delay each request for 'DownloadDelay' seconds to avoid blocking of some websites.
// If RandomizeDelay is true, each delay = delay * [0.5, 1.5)
type DelayMiddleware struct {
	BaseMiddleware
	DownloadDelay  float64
	RandomizeDelay bool
}

func (m *DelayMiddleware) ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error {
	delay := m.DownloadDelay
	if m.RandomizeDelay {
		delay *= rand.Float64() + 0.5
	}
	m.Logger.Debug(spider.Name, "Delay request %s for %.3f", req.URL, delay)
	time.Sleep(time.Duration(delay*1000) * time.Millisecond)
	return nil
}

// DepthMiddleware controls the max crawling depth of the spider.
// When DepthLimit is 0, there's no limitation.
type DepthMiddleware struct {
	BaseMiddleware
	DepthLimit int
}

func (m *DepthMiddleware) Open(spider *leiogo.Spider) error {
	m.Logger.Debug(spider.Name, "Init success with depthLimit: %d", m.DepthLimit)
	return nil
}

func (m *DepthMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	m.Logger.Debug(spider.Name, "Add depth meta to request %s", req.URL)
	if _, ok := res.Meta["depth"]; !ok {
		res.Meta["depth"] = 1
	}
	return nil
}

func (m *DepthMiddleware) ProcessNewRequest(req *leiogo.Request, parentRes *leiogo.Response, spider *leiogo.Spider) error {
	depth := parentRes.Meta["depth"].(int) + 1
	req.Meta["depth"] = depth
	m.Logger.Debug(spider.Name, "Depth of %s is %d", req.URL, depth)
	if m.DepthLimit != 0 && depth > m.DepthLimit {
		return &DropTaskError{Message: fmt.Sprintf("Depth beyond the max depth %d", m.DepthLimit)}
	}
	return nil
}

// HttpErrorMiddleware will drop all the responses with status code not 200.
type HttpErrorMiddleware struct {
	BaseMiddleware
}

func (m *HttpErrorMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	m.Logger.Debug(spider.Name, "Status code of %s: %d", req.URL, res.StatusCode)
	if res.StatusCode != 200 {
		return &DropTaskError{Message: fmt.Sprintf("[HTTP ERROR] %d", res.StatusCode)}
	}
	return nil
}

// OffSiteMiddleware will drop all the requests failing to match any AllowedDomain.
type OffSiteMiddleware struct {
	BaseMiddleware
}

func (m *OffSiteMiddleware) ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error {
	m.Logger.Debug(spider.Name, "Testing whether request %s off site", req.URL)
	if u, err := url.Parse(req.URL); err == nil {
		host := u.Host
		offsite := len(spider.AllowedDomains) != 0
		for _, domain := range spider.AllowedDomains {
			if strings.HasSuffix(host, domain) {
				m.Logger.Debug(spider.Name, "%s match domain: %s", req.URL, domain)
				offsite = false
				break
			}
		}
		if offsite {
			return &DropTaskError{Message: "Filtered off site request"}
		}
	}
	return nil
}

// When the downloader failed to download the request, retry middleware would put the request
// back to the task queue only if it hadn't reach the max retry times.
type RetryMiddleware struct {
	BaseMiddleware
	RetryEnabled bool
	RetryTimes   int
	Yielder
}

func (m *RetryMiddleware) Open(spider *leiogo.Spider) error {
	m.Logger.Debug(spider.Name, "Init success with retryEnanled: %v, retryTimes: %d", m.RetryEnabled, m.RetryTimes)
	return nil
}

func (m *RetryMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	if res.Err != nil {
		if m.isRetriable(req) {
			if err := m.NewRequest(req, nil, spider); err != nil {
				m.Logger.Error(spider.Name, "Add new request error, %s", err.Error())
			}
		}
		return &DropTaskError{Message: res.Err.Error()}
	}
	return nil
}

func (m *RetryMiddleware) isRetriable(req *leiogo.Request) bool {
	if m.RetryEnabled {
		if retry, ok := req.Meta["retry"]; ok && retry.(int) < m.RetryTimes {
			req.Meta["retry"] = retry.(int) + 1
			return true
		} else if !ok {
			req.Meta["retry"] = 1
			return true
		}
	}
	return false
}
