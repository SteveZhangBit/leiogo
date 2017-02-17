package middleware

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/garyburd/redigo/redis"

	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/log"
)

type Downloader interface {
	Download(req *leiogo.Request, spider *leiogo.Spider) (leioRes *leiogo.Response)
}

type ClientConfig interface {
	ConfigClient() (*http.Client, error)
}

// Usually we want to write the response entity to the file system, especially when we are
// creating a image download spider. But writing file system is a time-consuming work,
// we may add a memory cache layer in the middle. In order to keep the interface clean,
// the default downloader will own a file writer interface.
// The first string in the return values is to help logging.
type FileWriter interface {
	NotExists(filepath string) bool
	WriteFile(req *leiogo.Request, res *http.Response) (info string, writerErr error)
}

type FSWriter struct{}

func (f *FSWriter) NotExists(filepath string) bool {
	info, err := os.Stat(filepath)
	return os.IsNotExist(err) || info.Size() < 512
}

func (f *FSWriter) WriteFile(req *leiogo.Request, res *http.Response) (info string, writerErr error) {
	// Create a file from its filepath. We've already verified the request to be a file request
	// with type = file and filepath = 'path' in its meta
	filepath := req.Meta["__filepath__"].(string)
	if file, err := os.Create(filepath); err != nil {
		writerErr = err
	} else {
		// Create a counter to calculate the read content length.
		// This will compare to the Content-Length in the response header.
		var readLength int64 = 0

		// Read the response body and write it to file.
		buf := make([]byte, 4096)
		for {
			n, err := res.Body.Read(buf)

			// Pay attention that the read method in io.Reader will return n > 0
			// to indicate a successful read, and when it meets the file end, it will
			// return a EOF error. So it's possible that the n > 0 and an EOF error.
			if n > 0 {
				if _, err := file.Write(buf[:n]); err != nil {
					writerErr = err
					break
				}
				readLength += int64(n)
			}

			if err == io.EOF {
				// We want to drop this request after the download, so we create a drop task error here.
				// By default, the first download middleware it will meet is retry middleware,
				// and we have set an exception in the middleware, when it meets a drop task error,
				// it won't retry the request.
				writerErr = &DropTaskError{Message: "File download completed"}
				break
			} else if err != nil {
				writerErr = err
				break
			}
		}
		file.Close()

		if readLength == res.ContentLength {
			info = fmt.Sprintf("Saved %s to %s", req.URL, filepath)
		} else {
			writerErr = errors.New(fmt.Sprintf("Content length doesn't match, need %d, get %d", res.ContentLength, readLength))
			// Remove the imcompleted file
			os.Remove(filepath)
		}
	}
	return
}

// Downloader is where the requests truly be processed. It will execute the requests and produce
// the corresponding response.
type DefaultDownloader struct {
	// We use golang's builtin package to handle http requests.
	// In golang, we have to first create a httpclient object, and we can configure the client
	// with timeout, proxy and so on. Because this is the only difference among varied downloaders
	// (at least right now), so we just change the implemention of this interface.
	// See the implemention of DefaultConfig and ProxyConfig for more information.
	ClientConfig

	// We allowe users to set their custom User-Agent
	UserAgent string

	Logger log.Logger

	// From the page https://golang.org/pkg/net/http/#Client:
	// the Client's Transport typically has internal state (cached TCP connections),
	// so Clients should be reused instead of created as needed.
	// Clients are safe for concurrent use by multiple goroutines.
	client *http.Client

	// See the definition of FileWriter interface.
	FileWriter
}

func (d *DefaultDownloader) Download(req *leiogo.Request, spider *leiogo.Spider) (leioRes *leiogo.Response) {
	leioRes = leiogo.NewResponse(req)

	if retry, ok := req.Meta["retry"].(int); ok {
		d.Logger.Info(spider.Name, "Retrying %s for %d times", req.URL, retry)
	} else {
		d.Logger.Info(spider.Name, "Requesting %s", req.URL)
	}

	if enable, ok := req.Meta["phantomjs"]; ok && enable.(bool) {
		d.phantomjs(req, leioRes, spider)
	} else if typename, ok := req.Meta["__type__"].(string); ok && typename == "file" {
		d.fileDownload(req, leioRes, spider)
	} else {
		d.httpDownload(req, leioRes, spider)
	}

	return
}

func (d *DefaultDownloader) getResponse(req *leiogo.Request) (*http.Response, error) {
	if d.client == nil {
		var err error
		d.client, err = d.ConfigClient()
		if err != nil {
			return nil, err
		}
	}

	if getReq, err := http.NewRequest("GET", req.URL, nil); err != nil {
		return nil, err
	} else {
		if d.UserAgent != "" {
			getReq.Header.Set("User-Agent", d.UserAgent)
		}
		return d.client.Do(getReq)
	}
}

// The traditional way the handle http requests in golang.
func (d *DefaultDownloader) httpDownload(req *leiogo.Request, leioRes *leiogo.Response, spider *leiogo.Spider) {
	if res, err := d.getResponse(req); err != nil {
		leioRes.Err = err
	} else {
		// With the help of golang's defer feature, remember to close the response body.
		defer res.Body.Close()
		leioRes.StatusCode = res.StatusCode
		leioRes.Body, leioRes.Err = ioutil.ReadAll(res.Body)
	}
}

// We want the spider to have the ability to download files.
// Generally, we can directly download it from its url, but there are some problems.
// The first is that files are usually large, and if we get the response and read it into
// another byte array, we need a lot of memory which is not a godd idea.
// The second problem is that there's no need for the file to pass through the following middlewares,
// we want them to be writen into the target files as soon as possible.
func (d *DefaultDownloader) fileDownload(req *leiogo.Request, leioRes *leiogo.Response, spider *leiogo.Spider) {
	if res, err := d.getResponse(req); err != nil {
		leioRes.Err = err
	} else {
		// With the help of golang's defer feature, remember to close the response body.
		defer res.Body.Close()
		leioRes.StatusCode = res.StatusCode

		var info string
		info, leioRes.Err = d.WriteFile(req, res)
		if info != "" {
			d.Logger.Info(spider.Name, info)
		}
	}
}

// Add support for phantomjs. If user add 'phantomjs' = true to the requests' meta,
// such requests will be processed by phantomjs in a subprocess.
// Phantomjs is a headless webkit with javascript API, with its help,
// it's much more easy to handle the AJAX web pages.
// We are able to directly capture what we see on the browser, without site api digging.
func (d *DefaultDownloader) phantomjs(req *leiogo.Request, leioRes *leiogo.Response, spider *leiogo.Spider) {
	d.Logger.Info(spider.Name, "Using phantomjs for request %s", req.URL)

	// Using golang's exec package to run command, by default it will search the current directory,
	// so make sure to put phantomjs and download.js to the running directory.
	if out, err := exec.Command("phantomjs", "download.js", req.URL).Output(); err != nil {
		d.Logger.Error(spider.Name, "Exec error: %s", err.Error())
		leioRes.Err = err
	} else {
		if len(out) == 0 {
			leioRes.Err = errors.New("Phantomjs Error")
		} else {
			leioRes.Body = out

			// A request of a web page usually contains a bunch of related requests,
			// so it's not easy to define the status code of this request,
			// so we mistakely set it to 200.
			leioRes.StatusCode = 200
		}
	}
}

// We only config the timeout for the default config.
type DefaultConfig struct {
	Timeout int
}

func (c *DefaultConfig) ConfigClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: time.Duration(c.Timeout) * time.Second,
		Jar:     jar,
	}
	return client, nil
}

// Add proxy support to the downloader.
type ProxyConfig struct {
	Timeout  int
	ProxyURL string
}

func defaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func (c *ProxyConfig) ConfigClient() (*http.Client, error) {
	var proxyURL *url.URL
	var jar *cookiejar.Jar
	var err error

	jar, err = cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	proxyURL, err = url.Parse(c.ProxyURL)
	if err != nil {
		return nil, err
	}

	transport := defaultTransport()
	transport.Proxy = http.ProxyURL(proxyURL)

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(c.Timeout) * time.Second,
		Jar:       jar,
	}

	return client, nil
}

type RedisWriter struct {
	Addr string
}

func (r *RedisWriter) NotExists(filepath string) bool {
	if conn, err := redis.Dial("tcp", r.Addr); err != nil {
		return true
	} else {
		defer conn.Close()

		exists, err := conn.Do("EXISTS", filepath)
		return err != nil || exists.(int64) != 1
	}
}

func (r *RedisWriter) WriteFile(req *leiogo.Request, res *http.Response) (info string, writerErr error) {
	filepath := req.Meta["__filepath__"].(string)

	var conn redis.Conn
	conn, writerErr = redis.Dial("tcp", r.Addr)
	if writerErr != nil {
		return
	}
	defer conn.Close()

	var body []byte
	body, writerErr = ioutil.ReadAll(res.Body)
	if writerErr != nil {
		return
	}

	_, writerErr = conn.Do("SET", filepath, body)
	if writerErr != nil {
		return
	}
	writerErr = &DropTaskError{Message: "File cached completed"}
	return fmt.Sprintf("Cached %s to redis at %s", filepath, r.Addr), writerErr
}

func NewRedisWriter(addr string) *RedisWriter {
	return &RedisWriter{Addr: addr}
}
