package middleware

import (
	"errors"
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/log"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

type Downloader interface {
	Download(req *leiogo.Request, spider *leiogo.Spider) (leioRes *leiogo.Response)
}

type ClientConfig interface {
	ConfigClient() (*http.Client, error)
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

	Logger log.Logger
}

func (d *DefaultDownloader) Download(req *leiogo.Request, spider *leiogo.Spider) (leioRes *leiogo.Response) {
	leioRes = &leiogo.Response{Meta: req.Meta}

	if retry, ok := req.Meta["retry"].(int); ok {
		d.Logger.Info(spider.Name, "Retrying %s for %d times", req.URL, retry)
	} else {
		d.Logger.Info(spider.Name, "Requesting %s", req.URL)
	}

	if enable, ok := req.Meta["phantomjs"]; ok && enable.(bool) {
		d.phantomjs(req, leioRes, spider)
	} else if typename, ok := req.Meta["type"].(string); ok && typename == "video" {
		d.videoDownload(req, leioRes, spider)
	} else {
		d.httpDownload(req, leioRes, spider)
	}

	return
}

// The traditional way the handle http requests in golang.
func (d *DefaultDownloader) httpDownload(req *leiogo.Request, leioRes *leiogo.Response, spider *leiogo.Spider) {
	if client, err := d.ConfigClient(); err == nil {
		if res, err := client.Get(req.URL); err != nil {
			leioRes.Err = err
		} else {
			// With the help of golang's defer feature, remember to close the response body.
			defer res.Body.Close()

			leioRes.StatusCode = res.StatusCode
			leioRes.Body, leioRes.Err = ioutil.ReadAll(res.Body)
		}
	} else {
		leioRes.Err = err
	}
}

// We want the spider to have the ability to download video files.
// Generally, we can directly download it from its url, but there are some problems.
// The first is that video files are usually large, and if we get the response and read it into
// another byte array, we need a lot of memory which is not a godd idea.
// The second problem is that there's no need for video file to pass through the following middlewares.
// This is similar to images, but images are usually small, we can bear the price.
// But for video files, we want them to be writen into the target files as soon as possible.
func (d *DefaultDownloader) videoDownload(req *leiogo.Request, leioRes *leiogo.Response, spider *leiogo.Spider) {
	if client, err := d.ConfigClient(); err == nil {
		if res, err := client.Get(req.URL); err != nil {
			leioRes.Err = err
		} else {
			// With the help of golang's defer feature, remember to close the response body.
			defer res.Body.Close()

			leioRes.StatusCode = res.StatusCode
			d.writeVideo(req, res, leioRes)
		}
	} else {
		leioRes.Err = err
	}
}

func (d *DefaultDownloader) writeVideo(req *leiogo.Request, res *http.Response, leioRes *leiogo.Response) {
	// Create a file from its filepath. We've already verified the request to be a video request
	// with type = video and filepath = 'path' in its meta
	if file, err := os.Create(req.Meta["filepath"].(string)); err != nil {
		leioRes.Err = err
	} else {
		defer file.Close()

		// Read the response body and write it to file.
		buf := make([]byte, 4096)
		for {
			if _, err := res.Body.Read(buf); err != nil {
				leioRes.Err = err
				break
			} else if err == io.EOF {
				// We want to drop this request after the download, so we create a drop task error here.
				// By default, the first download middleware it will meet is retry middleware,
				// and we have set an exception in the middleware, when it meets a drop task error,
				// it won't retry the request.
				leioRes.Err = &DropTaskError{Message: "Video download success"}
				break
			} else {
				if _, err := file.Write(buf); err != nil {
					leioRes.Err = err
					break
				}
			}
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

// We only config the timeout for the default config.
type DefaultConfig struct {
	Timeout int
}

func (c *DefaultConfig) ConfigClient() (*http.Client, error) {
	transport := defaultTransport()

	transport.DialContext = (&net.Dialer{
		Timeout:   time.Duration(c.Timeout) * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext

	client := &http.Client{
		Transport: transport,
	}
	return client, nil
}

// Add proxy support to the downloader.
type ProxyConfig struct {
	Timeout  int
	ProxyURL string
}

func (c *ProxyConfig) ConfigClient() (*http.Client, error) {
	if proxyURL, err := url.Parse(c.ProxyURL); err == nil {
		transport := defaultTransport()

		transport.DialContext = (&net.Dialer{
			Timeout:   time.Duration(c.Timeout) * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext

		transport.Proxy = http.ProxyURL(proxyURL)

		client := &http.Client{
			Transport: transport,
		}

		return client, nil
	} else {
		return nil, err
	}
}
