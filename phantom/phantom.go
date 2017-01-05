package phantom

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/log"
	"os/exec"
)

type PhantomDownloader struct {
	Logger log.Logger
}

func (p *PhantomDownloader) Download(req *leiogo.Request, spider *leiogo.Spider) (leioRes *leiogo.Response) {
	type PhantomRes struct {
		Err  string
		Body string
	}

	leioRes = &leiogo.Response{Meta: req.Meta}

	p.Logger.Info(spider.Name, "Start download %s using phantomjs", req.URL)

	if out, err := exec.Command("phantomjs.exe", "download.js", req.URL).Output(); err != nil {
		p.Logger.Error(spider.Name, "Exec error: %s", err.Error())
		leioRes.Err = err
	} else {
		var res PhantomRes

		dec := json.NewDecoder(bytes.NewReader(out))
		if err := dec.Decode(&res); err != nil {
			p.Logger.Error(spider.Name, "JSON decode error: ", err.Error())
			leioRes.Err = err
		} else if res.Err != "" {
			leioRes.Err = errors.New(res.Err)
		} else {
			leioRes.Body = ([]byte)(res.Body)
			leioRes.StatusCode = 200
		}
	}
	return
}
