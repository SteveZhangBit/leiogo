package middleware

import (
	"github.com/SteveZhangBit/leiogo"
)

type Parser func(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider)
