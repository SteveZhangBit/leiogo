package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/log"
	"io/ioutil"
	"net/http"
	"net/url"
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

// Add support for phantomjs. If user add 'phantomjs' = true to the requests' meta,
// such requests will be processed by phantomjs in a subprocess.
// Phantomjs is a headless webkit with javascript API, with its help,
// it's much more easy to handle the AJAX web pages.
// We are able to directly capture what we see on the browser, without site api digging.
func (d *DefaultDownloader) phantomjs(req *leiogo.Request, leioRes *leiogo.Response, spider *leiogo.Spider) {
	// download.js will return a json string with error and body to the stdout.
	// So we have to read it from stdout and transfer it to json object, and then write it to leiogo.Response.
	type PhantomRes struct {
		Err  string
		Body string
	}

	d.Logger.Info(spider.Name, "Using phantomjs for request %s", req.URL)
	// Using golang's exec package to run command, by default it will search the current directory,
	// so make sure to put phantomjs and download.js to the running directory.
	if out, err := exec.Command("phantomjs", "download.js", req.URL).Output(); err != nil {
		d.Logger.Error(spider.Name, "Exec error: %s", err.Error())
		leioRes.Err = err
	} else {
		var res PhantomRes

		dec := json.NewDecoder(bytes.NewReader(out))
		if err := dec.Decode(&res); err != nil {
			d.Logger.Error(spider.Name, "JSON decode error: ", err.Error())
			leioRes.Err = err
		} else if res.Err != "" {
			leioRes.Err = errors.New(res.Err)
		} else {
			leioRes.Body = ([]byte)(res.Body)
			// Nowadays, a request of a web page usually contains a bunch of related requests,
			// so it's not easy to define the status code of this request,
			// so we mistakely set it to 200.
			leioRes.StatusCode = 200
		}
	}
}

// We only config the timeout for the default config.
type DefaultConfig struct {
	Timeout time.Duration
}

func (c *DefaultConfig) ConfigClient() (*http.Client, error) {
	return &http.Client{Timeout: c.Timeout * time.Second}, nil
}

// Add proxy support to the downloader.
type ProxyConfig struct {
	Timeout  time.Duration
	ProxyURL string
}

func (c *ProxyConfig) ConfigClient() (*http.Client, error) {
	if proxyURL, err := url.Parse(c.ProxyURL); err == nil {
		return &http.Client{
				Timeout:   c.Timeout * time.Second,
				Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
			},
			nil
	} else {
		return nil, err
	}
}
