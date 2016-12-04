package leiogo

import (
	"github.com/satori/go.uuid"
)

type Spider struct {
	Name           string
	StartURLs      []*Request
	AllowedDomains []string
}

type Request struct {
	URL        string
	Meta       map[string]interface{}
	ParserName string
}

func NewRequest(url string) *Request {
	return &Request{
		URL:        url,
		Meta:       make(map[string]interface{}),
		ParserName: "default",
	}
}

type Response struct {
	Err        error
	StatusCode int
	Body       []byte
	Meta       map[string]interface{}
}

type Item struct {
	ID   string
	Data map[string]interface{}
}

func NewItem(data map[string]interface{}) *Item {
	return &Item{
		ID:   uuid.NewV4().String(),
		Data: data,
	}
}

func (i *Item) String() string {
	return i.ID
}
