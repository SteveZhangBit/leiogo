package middleware

import (
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/SteveZhangBit/leiogo"
	"io"
	"os"
	"path"
	"strings"
)

type ItemPipeline interface {
	OpenClose
	Process(item *leiogo.Item, spider *leiogo.Spider) error
	HandleErr
}

type DropItemError struct {
	Message string
}

func (err *DropItemError) Error() string {
	return err.Message
}

type ImagePipeline struct {
	Base
	DirPath string
	Yielder
}

func (p *ImagePipeline) Open(spider *leiogo.Spider) error {
	p.Logger.Debug(spider.Name, "Init success with file directory: %s", p.DirPath)
	return nil
}

func (p *ImagePipeline) Process(item *leiogo.Item, spider *leiogo.Spider) error {
	subpath := p.DirPath
	if filepath, ok := item.Data["filepath"].(string); ok {
		subpath = path.Join(p.DirPath, filepath)
	}
	if err := os.MkdirAll(subpath, os.ModeDir); err != nil {
		p.Logger.Error(spider.Name, "Create directory failed, %s", err.Error())
	}

	for _, url := range item.Data["fileurls"].([]string) {
		ext := url[strings.LastIndex(url, "."):]
		filepath := path.Join(subpath, hashURL(url)+ext)
		if info, err := os.Stat(filepath); os.IsNotExist(err) || info.Size() < 512 {
			imgRequest := leiogo.NewRequest(url)
			imgRequest.Meta["type"] = "file"
			imgRequest.Meta["filepath"] = filepath

			if err := p.NewRequest(imgRequest, nil, spider); err != nil {
				p.Logger.Error(spider.Name, "Add img request error %s", err.Error())
			}
		}
	}
	return nil
}

func hashURL(input string) string {
	h := md5.New()
	io.WriteString(h, input)
	return fmt.Sprintf("%x", h.Sum(nil))
}

type SaveImageMiddleware struct {
	BaseMiddleware
}

func (m *SaveImageMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	_, typeOk := res.Meta["type"]
	filepath, pathOK := res.Meta["filepath"].(string)

	if typeOk && pathOK {
		m.Logger.Info(spider.Name, "Saving %s to %s", req.URL, filepath)

		if f, err := os.Create(filepath); err == nil {
			defer f.Close()
			if _, err := f.Write(res.Body); err != nil {
				return errors.New(fmt.Sprintf("Saving %s fail, %s", req.URL, err.Error()))
			} else {
				return &DropTaskError{Message: "Saving image completed"}
			}
		} else {
			return err
		}
	}

	return nil
}
