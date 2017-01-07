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

// When a middleware wants to drop the current task, return this type of error.
// We are able to add drop details to the Message field.
type DropTaskError struct {
	Message string
}

func (err *DropTaskError) Error() string {
	return err.Message
}

// CacheMiddleware is a download middleware.
// Using CacheMiddleware to store the crawled urls and avoid duplicated urls.
// Cause each middleware will be called in different goroutines, so Locking is necessary.
type CacheMiddleware struct {
	BaseMiddleware

	// We simply use a dictionary to store the requested urls,
	// considering the memory usage, we make the value to be struct{},
	// in golang it will use 0 space.
	Cache map[string]struct{}

	// We use a RWMutex here, instead of the Mutex struct.
	mutex sync.RWMutex
}

// First lock the mutex, then test whether the url has cached, if it is, then drop it.
// Pay attention that because we only need to read from the cache, so we should call
// RWMutex's RLock method.
func (m *CacheMiddleware) ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	m.Logger.Debug(spider.Name, "Test whether %s is cached", req.URL)
	if _, ok := m.Cache[req.URL]; ok {
		return &DropTaskError{Message: "URL already parsed"}
	}
	return nil
}

// First lock the mutex, then add the url into the cache,
// pay attention that we need to call the RWMutex's Lock method,
// because we have to write the cache.
func (m *CacheMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.Logger.Debug(spider.Name, "Add %s to cache", req.URL)
	m.Cache[req.URL] = struct{}{}
	return nil
}

// DelayMiddleware is a download middleware.
// Delay each request for 'DownloadDelay' seconds to avoid blocking of some websites.
// If RandomizeDelay is true, each delay = delay * [0.5, 1.5)
type DelayMiddleware struct {
	BaseMiddleware

	// DownloadDelay defines the basic delay seconds for each request,
	// the default value is set to 2.0s, see the definition in crawler package.
	DownloadDelay float64

	// Randomize the delay seconds, the default range is from 0.5 times to 1.5 times.
	RandomizeDelay bool
}

func (m *DelayMiddleware) ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error {
	delay := m.DownloadDelay
	if m.RandomizeDelay {
		delay *= rand.Float64() + 0.5
	}
	m.Logger.Debug(spider.Name, "Delay request %s for %.3f", req.URL, delay)

	// We simply use time.Sleep to make the goroutine to wait for the demanding seconds.
	// Since each request is processed in a seperate goroutine, so don't worry it will block the main thread.
	time.Sleep(time.Duration(delay*1000) * time.Millisecond)
	return nil
}

// DepthMiddleware is a spider middleware.
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

// We simply store the depth information in the request's and response's meta,
// and since that we will copy the meta information of a request to its corresponding response,
// therefore all the requests and the response must carry the depth information.
// However, there's one exception. For the start requests (built from startURLs), we didn't add
// depth information to it, so we have to set the depth to 1 for those responses without depth information.
// In general, this would only happen to start requests.
func (m *DepthMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	m.Logger.Debug(spider.Name, "Add depth meta to request %s", req.URL)

	if _, ok := res.Meta["depth"]; !ok {
		res.Meta["depth"] = 1
	}
	return nil
}

// Because we must set the depth information to all the new requests,
// so we simply read the depth information from the parent response and add it to the new request.
// And if the DepthLimit is not 0, meaning that there is a limitation,
// and if the depth of the new request beyond the max depth, then drop the request.
func (m *DepthMiddleware) ProcessNewRequest(req *leiogo.Request, parentRes *leiogo.Response, spider *leiogo.Spider) error {
	depth := parentRes.Meta["depth"].(int) + 1
	req.Meta["depth"] = depth
	m.Logger.Debug(spider.Name, "Depth of %s is %d", req.URL, depth)
	if m.DepthLimit != 0 && depth > m.DepthLimit {
		return &DropTaskError{Message: fmt.Sprintf("Depth beyond the max depth %d", m.DepthLimit)}
	}
	return nil
}

// HttpErrorMiddleware is a spider middleware (well, in fact we only define its ProcessResponse method,
// we say it a spider middleware only because we want to make it happen after all those download middlwares).
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

// OffSiteMiddleware is a download middleware.
// OffSiteMiddleware will drop all the requests failing to match any AllowedDomain.
type OffSiteMiddleware struct {
	BaseMiddleware
}

func (m *OffSiteMiddleware) ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error {
	m.Logger.Debug(spider.Name, "Testing whether request %s off site", req.URL)
	if u, err := url.Parse(req.URL); err == nil {

		// Create an url object from the url string in order to get the host name.
		host := u.Host

		// If spider's AllowedDomains field is empty, it should always pass this middleware.
		offsite := len(spider.AllowedDomains) != 0

		// Traverse all the domains, if there's one that can match the request url,
		// then set offsite to false.
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

// RetryMiddleware is a download middlware.
// When the downloader failed to download the request, retry middleware would put the request
// back to the task queue only if it hadn't reach the max retry times.
type RetryMiddleware struct {
	BaseMiddleware

	// Retry will happen only if this field is set to true and the default value is true.
	// See the definition of the default value in crawler package.
	RetryEnabled bool

	// This field defines the max retry times for each request.
	// The default value is set to 3, see the definition in crawler package.
	RetryTimes int

	Yielder
}

func (m *RetryMiddleware) Open(spider *leiogo.Spider) error {
	m.Logger.Debug(spider.Name, "Init success with retryEnanled: %v, retryTimes: %d", m.RetryEnabled, m.RetryTimes)
	return nil
}

func (m *RetryMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	// Retry will occur only if the Err field of the response is not nil.
	// And it usually should be a connection error.
	// Pay attention to an exception, we add file download feature to our downloader, and in order to
	// stop its spread to the following middlewares, we set a DropTaskError to the Err field.
	// In this situation, we don't need to retry.
	switch res.Err.(type) {
	case nil:
		return nil
	case *DropTaskError:
		return res.Err
	default:
		// Test whether this request is retriable, see the function below.
		if m.isRetriable(req) {
			if err := m.NewRequest(req, nil, spider); err != nil {
				m.Logger.Error(spider.Name, "Add new request error, %s", err.Error())
			}
		}
		return &DropTaskError{Message: res.Err.Error()}
	}
}

// A request is retriable when RetryEnabled is set to true and the retry times of this request
// havn't reach the max retry times.
// And we simply store the retry information in the request's meta.
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
