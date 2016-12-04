package proxy

import (
	"fmt"
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/middleware"
	"net"
	"net/rpc"
)

func Dial(url string, call func(client *rpc.Client) error) error {
	client, err := rpc.Dial("tcp", url)
	if err != nil {
		return err
	}
	defer client.Close()
	return call(client)
}

func Serve(srvc interface{}, port string) {
	rpc.Register(srvc)
	if listen, err := net.Listen("tcp", port); err != nil {
		fmt.Errorf("Failed to start rpc server on %s for service %T, %s", port, srvc, err.Error())
	} else {
		for {
			if conn, err := listen.Accept(); err != nil {
				fmt.Errorf("Error at accepting rpc connection, %s", err.Error())
				return
			} else {
				go rpc.ServeConn(conn)
			}
		}
	}
}

type CloseArgs struct {
	Reason string
	Spider *leiogo.Spider
}

type ErrArgs struct {
	Err    error
	Spider *leiogo.Spider
}

type ReqArgs struct {
	Req    *leiogo.Request
	Spider *leiogo.Spider
}

type ResArgs struct {
	Req    *leiogo.Request
	Res    *leiogo.Response
	Spider *leiogo.Spider
}

type ItemArgs struct {
	Item   *leiogo.Item
	Spider *leiogo.Spider
}

type YielderProxy struct {
	URL string
}

func (y *YielderProxy) NewRequest(req *leiogo.Request, parRes *leiogo.Response, spider *leiogo.Spider) error {
	args := ResArgs{Req: req, Res: parRes, Spider: spider}
	return Dial(y.URL, func(client *rpc.Client) error {
		return client.Call("YielderServer.NewRequest", args, &struct{}{})
	})
}

func (y *YielderProxy) NewItem(item *leiogo.Item, spider *leiogo.Spider) error {
	args := ItemArgs{Item: item, Spider: spider}
	return Dial(y.URL, func(client *rpc.Client) error {
		return client.Call("YielderServer.NewItem", args, &struct{}{})
	})
}

type YielderServer struct {
	Yielder middleware.Yielder
}

func (y *YielderServer) NewRequest(args ResArgs, _ *struct{}) error {
	y.Yielder.NewRequest(args.Req, args.Res, args.Spider)
	return nil
}

func (y *YielderServer) NewItem(args ItemArgs, _ *struct{}) error {
	y.Yielder.NewItem(args.Item, args.Spider)
	return nil
}

type BaseProxy struct {
	URL      string
	SrvcName string
}

func (d *BaseProxy) Open(spider *leiogo.Spider) error {
	return Dial(d.URL, func(client *rpc.Client) error {
		return client.Call(d.SrvcName+".Open", spider, &struct{}{})
	})
}

func (d *BaseProxy) Close(reason string, spider *leiogo.Spider) error {
	args := CloseArgs{Reason: reason, Spider: spider}
	return Dial(d.URL, func(client *rpc.Client) error {
		return client.Call(d.SrvcName+".Close", args, &struct{}{})
	})
}

func (d *BaseProxy) HandleErr(err error, spider *leiogo.Spider) {
	args := ErrArgs{Err: err, Spider: spider}
	Dial(d.URL, func(client *rpc.Client) error {
		return client.Call(d.SrvcName+".HandlerErr", args, &struct{}{})
	})
}

type MiddlewareProxy struct {
	BaseProxy
}

func (m *MiddlewareProxy) ProcessRequest(req *leiogo.Request, spider *leiogo.Spider) error {
	args := ReqArgs{Req: req, Spider: spider}
	return Dial(m.URL, func(client *rpc.Client) error {
		return client.Call(m.SrvcName+".ProcessRequest", args, &struct{}{})
	})
}

func (m *MiddlewareProxy) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	args := ResArgs{Req: req, Res: res, Spider: spider}
	return Dial(m.URL, func(client *rpc.Client) error {
		return client.Call(m.SrvcName+".ProcessResponse", args, &struct{}{})
	})
}

func (m *MiddlewareProxy) ProcessNewRequest(req *leiogo.Request, parentRes *leiogo.Response, spider *leiogo.Spider) error {
	args := ResArgs{Req: req, Res: parentRes, Spider: spider}
	return Dial(m.URL, func(client *rpc.Client) error {
		return client.Call(m.SrvcName+".ProcessNewRequest", args, &struct{}{})
	})
}

type ItemPipelineProxy struct {
	BaseProxy
}

func (i *ItemPipelineProxy) Process(item *leiogo.Item, spider *leiogo.Spider) error {
	args := ItemArgs{Item: item, Spider: spider}
	return Dial(i.URL, func(client *rpc.Client) error {
		return client.Call(i.SrvcName+".Process", args, &struct{}{})
	})
}

type DownloaderProxy struct {
	URL string
}

func (d *DownloaderProxy) Download(req *leiogo.Request, spider *leiogo.Spider) (leioRes *leiogo.Response) {
	args := ReqArgs{Req: req, Spider: spider}
	leioRes = &leiogo.Response{}
	err := Dial(d.URL, func(client *rpc.Client) error {
		return client.Call("DownloaderServer.Download", args, leioRes)
	})
	if err != nil {
		leioRes.Err = err
	}
	return
}

type OpenCloseServer struct {
	OpenClose middleware.OpenClose
}

func (o *OpenCloseServer) Open(spider *leiogo.Spider, _ *struct{}) error {
	return o.OpenClose.Open(spider)
}

func (o *OpenCloseServer) Close(args CloseArgs, _ *struct{}) error {
	return o.OpenClose.Close(args.Reason, args.Spider)
}

type HandleErrServer struct {
	Handler middleware.HandleErr
}

func (h *HandleErrServer) HandleErr(args ErrArgs, _ *struct{}) error {
	h.Handler.HandleErr(args.Err, args.Spider)
	return nil
}

type DownloadMiddlewareServer struct {
	OpenCloseServer
	HandleErrServer
	Middleware middleware.DownloadMiddleware
}

func (d *DownloadMiddlewareServer) ProcessRequest(args ReqArgs, _ *struct{}) error {
	return d.Middleware.ProcessRequest(args.Req, args.Spider)
}

func (d *DownloadMiddlewareServer) ProcessResponse(args ResArgs, _ *struct{}) error {
	return d.Middleware.ProcessResponse(args.Res, args.Req, args.Spider)
}

type SpiderMiddlewareServer struct {
	OpenCloseServer
	HandleErrServer
	Middleware middleware.SpiderMiddleware
}

func (s *SpiderMiddlewareServer) ProcessResponse(args ResArgs, _ *struct{}) error {
	return s.Middleware.ProcessResponse(args.Res, args.Req, args.Spider)
}

func (s *SpiderMiddlewareServer) ProcessNewRequest(args ResArgs, _ *struct{}) error {
	return s.Middleware.ProcessNewRequest(args.Req, args.Res, args.Spider)
}

type ItemPipelineServer struct {
	OpenCloseServer
	HandleErrServer
	Pipeline middleware.ItemPipeline
}

func (i *ItemPipelineServer) Process(args ItemArgs, _ *struct{}) error {
	return i.Pipeline.Process(args.Item, args.Spider)
}

type DownloaderServer struct {
	Downloader middleware.Downloader
}

func (d *DownloaderServer) Download(args ReqArgs, leioRes *leiogo.Response) error {
	*leioRes = *d.Downloader.Download(args.Req, args.Spider)
	return nil
}

func NewYielderProxy(url string) middleware.Yielder {
	return &YielderProxy{URL: url}
}

func NewDownloadMiddlewareProxy(url string) middleware.DownloadMiddleware {
	return &MiddlewareProxy{BaseProxy: BaseProxy{URL: url, SrvcName: "DownloadMiddlewareServer"}}
}

func NewSpiderMiddlewareProxy(url string) middleware.SpiderMiddleware {
	return &MiddlewareProxy{BaseProxy: BaseProxy{URL: url, SrvcName: "SpiderMiddlewareServer"}}
}

func NewItemPipelineProxy(url string) middleware.ItemPipeline {
	return &ItemPipelineProxy{BaseProxy: BaseProxy{URL: url, SrvcName: "ItemPipelineServer"}}
}

func NewDownloaderProxy(url string) middleware.Downloader {
	return &DownloaderProxy{URL: url}
}

func NewYielderServer(yielder middleware.Yielder) *YielderServer {
	return &YielderServer{Yielder: yielder}
}

func NewDownloadMiddlewareServer(m middleware.DownloadMiddleware) *DownloadMiddlewareServer {
	return &DownloadMiddlewareServer{
		OpenCloseServer: OpenCloseServer{OpenClose: m},
		HandleErrServer: HandleErrServer{Handler: m},
		Middleware:      m,
	}
}

func NewSpiderMiddlewareServer(m middleware.SpiderMiddleware) *SpiderMiddlewareServer {
	return &SpiderMiddlewareServer{
		OpenCloseServer: OpenCloseServer{OpenClose: m},
		HandleErrServer: HandleErrServer{Handler: m},
		Middleware:      m,
	}
}

func NewItemPipelineServer(p middleware.ItemPipeline) *ItemPipelineServer {
	return &ItemPipelineServer{
		OpenCloseServer: OpenCloseServer{OpenClose: p},
		HandleErrServer: HandleErrServer{Handler: p},
		Pipeline:        p,
	}
}

func NewDownloaderServer(d middleware.Downloader) *DownloaderServer {
	return &DownloaderServer{Downloader: d}
}
