package middleware

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/log"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Downloader interface {
	Download(req *leiogo.Request, spider *leiogo.Spider) (leioRes *leiogo.Response)
}

type ClientConfig interface {
	ConfigClient() (*http.Client, error)
}

type DefaultDownloader struct {
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

	if client, err := d.ConfigClient(); err == nil {
		if res, err := client.Get(req.URL); err != nil {
			leioRes.Err = err
		} else {
			leioRes.StatusCode = res.StatusCode
			leioRes.Body, leioRes.Err = ioutil.ReadAll(res.Body)
			res.Body.Close()
		}
	} else {
		leioRes.Err = err
	}

	return
}

type DefaultConfig struct {
	Timeout time.Duration
}

func (c *DefaultConfig) ConfigClient() (*http.Client, error) {
	return &http.Client{Timeout: c.Timeout * time.Second}, nil
}

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
