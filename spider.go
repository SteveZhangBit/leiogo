package leiogo

import (
	"github.com/satori/go.uuid"
)

type Dict map[string]interface{}

type Spider struct {
	Name           string
	StartURLs      []*Request
	AllowedDomains []string
}

type Request struct {
	URL        string
	Meta       Dict
	ParserName string
}

func NewRequest(url string) *Request {
	return &Request{
		URL:        url,
		Meta:       make(Dict),
		ParserName: "default",
	}
}

type Response struct {
	Err        error
	StatusCode int
	Body       []byte
	Meta       Dict
}

type Item struct {
	ID   string
	Data Dict
}

func NewItem(data Dict) *Item {
	return &Item{
		ID:   uuid.NewV4().String(),
		Data: data,
	}
}

func (i *Item) String() string {
	return i.ID
}
