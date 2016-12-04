package middleware

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/log"
)

type OpenClose interface {
	Open(spider *leiogo.Spider) error
	Close(reason string, spider *leiogo.Spider) error
}

type HandleErr interface {
	HandleErr(err error, spider *leiogo.Spider)
}

type DownloadMiddleware interface {
	OpenClose
	ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error
	ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error
	HandleErr
}

type SpiderMiddleware interface {
	OpenClose
	ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error
	ProcessNewRequest(req *leiogo.Request, parentRes *leiogo.Response, spider *leiogo.Spider) error
	HandleErr
}

type Yielder interface {
	NewRequest(req *leiogo.Request, parRes *leiogo.Response, spider *leiogo.Spider)
	NewItem(item *leiogo.Item, spider *leiogo.Spider)
}

type Base struct {
	Logger log.Logger
}

func (b *Base) Open(spider *leiogo.Spider) error {
	b.Logger.Debug(spider.Name, "Init success")
	return nil
}

func (b *Base) Close(reason string, spider *leiogo.Spider) error {
	b.Logger.Debug(spider.Name, "Close success")
	return nil
}

func (b *Base) HandleErr(err error, spider *leiogo.Spider) {
	b.Logger.Error(spider.Name, "%s", err.Error())
}

type BaseMiddleware struct{}

func (b *BaseMiddleware) ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error {
	return nil
}

func (b *BaseMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	return nil
}

func (b *BaseMiddleware) ProcessNewRequest(req *leiogo.Request, parentRes *leiogo.Response, spider *leiogo.Spider) error {
	return nil
}
