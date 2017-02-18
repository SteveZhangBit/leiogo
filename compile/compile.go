package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const MainTemplate = `
package main

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo-css/selector"
	"github.com/SteveZhangBit/leiogo/crawler"
	"net/url"
)

// user defined imports
%s

// user defined vars
var (
%s
)

func init() {
// config crawler
%s

// config logger
%s
}

type Parser struct {
	crawler.DefaultParser
}

// User defined parser functions
%s

// main function
func main() {
// config spider
%s

// config builder
builder := crawler.DefaultCrawlerBuilder()
%s

// config parser to builder
parser := &Parser{DefaultParser: builder.DefaultParser()}
%s

// build and run
builder.Build().Crawl(spider)
}
`

const ParseFuncTemplate = `
func (p *Parser) %s(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) {
// user defined variables
%s

// user defined patterns
patterns := map[string]crawler.PatternFunc{}

%s

p.RunPattern(patterns, res, spider)
}
`

const PatternFuncTemplate = `
patterns["%s"] = func(el *selector.Elements) []interface{} {
var products []interface{}

// add item(s) and/or request(s) here
%s

return products
}
`

var (
	CodeImports   = ""
	CodeVars      = ""
	CodeFunctions = ""
	CodeCrawler   = ""
	CodeLogger    = ""
	CodeSpider    = ""
	CodeBuilder   = ""
	CodeParser    = ""
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("The compiler needs a file. Usage: compile filename.json")
		return
	}

	if data, err := ioutil.ReadFile(os.Args[1]); err != nil {
		fmt.Println("File read error: ", err)
	} else {
		var dic map[string]interface{}

		if err := json.Unmarshal(data, &dic); err != nil {
			fmt.Println("JSON decode error: ", err)
		} else {
			// We define several different keywords.

			for key, val := range dic {
				switch key {

				// "imports" indicates the user defined imports
				case "imports":
					ConfigImports(val.([]interface{}))

				// "vars" defines the user defined global variables.
				case "vars":
					ConfigVars(val.([]interface{}))

				// "crawler" indicates the crawler package. We have defined some
				// const in the package, like DepthLimit, RetryTimes.
				case "crawler":
					ConfigCrawler(val.(map[string]interface{}))

				// "log" indicates the logger package, users can change the loglevel
				// among "Fatal", "Error", "Info", "Debug", "Trace".
				case "log":
					ConfigLogger(val.(string))

				// "spider" indicates the spider which the user wants to create, it should
				// be a json object including Name, StartURLs and AllowedDomains.
				case "spider":
					ConfigSpider(val.(map[string]interface{}))

				// "builder" is used to help us config the crawler components. The key should
				// be the function name like SetDownloader, and the value is the demanding parameters.
				case "builder":
					ConfigBuilder(val.(map[string]interface{}))

				// The rest will all be treated as parsers, and there should be at least one parser named "parser"
				default:
					ConfigParser(key, val.(map[string]interface{}))
				}
			}

			target, _ := os.Create(os.Args[1] + ".go")
			fmt.Fprintf(target, MainTemplate,
				CodeImports,
				CodeVars,
				CodeCrawler,
				CodeLogger,
				CodeFunctions,
				CodeSpider,
				CodeBuilder,
				CodeParser)
			target.Close()

			// Use gofmt to format the code, make it more readable.
			exec.Command("go", "fmt", os.Args[1]+".go").Start()
			exec.Command("goimports", "-w", os.Args[1]+".go").Start()
		}
	}
}

func ConfigImports(a []interface{}) {
	for _, val := range a {
		CodeImports += fmt.Sprintf("import \"%s\"\n", val.(string))
	}
}

func ConfigVars(a []interface{}) {
	for _, i := range a {
		for key, val := range i.(map[string]interface{}) {
			CodeVars += fmt.Sprintf("%s = %v\n", key, eval(val))
		}
	}
}

func ConfigCrawler(dic map[string]interface{}) {
	for key, val := range dic {
		CodeCrawler += fmt.Sprintf("crawler.%s = %v\n", key, eval(val))
	}
}

func ConfigLogger(level string) {
	CodeLogger = fmt.Sprintf("log.LogLevel = log.%s", level)
	CodeImports += "import \"github.com/SteveZhangBit/leiogo/log\"\n"
}

func ConfigSpider(dic map[string]interface{}) {
	CodeSpider = "spider := &leiogo.Spider{\n"
	for key, val := range dic {
		switch key {

		case "Name":
			CodeSpider += fmt.Sprintf("Name: %v,\n", eval(val))

		case "StartURLs":
			CodeSpider += "StartURLs: []*leiogo.Request{\n"
			for _, req := range val.([]interface{}) {
				CodeSpider += createRequest(req.(map[string]interface{})) + ",\n"
			}
			CodeSpider += "},\n"

		case "AllowedDomains":
			CodeSpider += fmt.Sprintf("AllowedDomains: []string%v,\n", eval(val))
		}
	}
	CodeSpider += "}\n"
}

func ConfigBuilder(dic map[string]interface{}) {
	CodeBuilder = "builder.\n"
	for key, val := range dic {
		CodeBuilder += fmt.Sprintf("%s(%v).\n", key, eval(val))
	}
	CodeBuilder = CodeBuilder[:len(CodeBuilder)-2]
}

func ConfigParser(name string, dic map[string]interface{}) {
	// Add parser name to builder
	funcName := strings.ToUpper(name[:1]) + name[1:]
	CodeParser += fmt.Sprintf("builder.AddParser(\"%s\", parser.%s)\n", name, funcName)

	// Generate functions to the Parser type
	patterns := ""
	vars := ""
	for key, val := range dic {
		if key == "vars" {
			vars = createPatternVars(val.([]interface{}))
		} else {
			patterns += fmt.Sprintf(PatternFuncTemplate, key, createPatternFunc(val.(map[string]interface{})))
		}
	}

	CodeFunctions += fmt.Sprintf(ParseFuncTemplate, funcName, vars, patterns)
}

func createPatternVars(a []interface{}) (code string) {
	for _, dic := range a {
		for name, val := range dic.(map[string]interface{}) {
			code += fmt.Sprintf("%s := %v\n", name, eval(val))
		}
	}
	return
}

func createPatternFunc(dic map[string]interface{}) (code string) {
	for key, val := range dic {
		switch key {

		case "vars":
			code = createPatternVars(val.([]interface{})) + code

		case "item":
			code += fmt.Sprintf("products = append(products, %s)\n", createItem(val.(map[string]interface{})))

		case "items":
			for _, item := range val.([]interface{}) {
				code += fmt.Sprintf("products = append(products, %s)\n", createItem(item.(map[string]interface{})))
			}

		case "request":
			code += fmt.Sprintf("products = append(products, %s)\n", createRequest(val.(map[string]interface{})))

		case "requests":
			for _, req := range val.([]interface{}) {
				code += fmt.Sprintf("products = append(products, %s)\n", createRequest(req.(map[string]interface{})))
			}

		case "if":
			for i, statement := range val.([]interface{}) {
				condition, body := createIfStatement(statement.(map[string]interface{}))
				if i == 0 {
					code += fmt.Sprintf("if %s {\n%s}", condition, body)
				} else if condition != "" {
					code += fmt.Sprintf(" else if %s {\n%s}", condition, body)
				} else {
					code += fmt.Sprintf(" else {\n%s}", body)
				}
			}

		case "lines":
			for _, line := range val.([]interface{}) {
				code += line.(string) + "\n"
			}

		default:
			// for loop pattern
			if regexp.MustCompile(`^for \w+, ?\w+ in .+`).MatchString(key) {
				code += fmt.Sprintf("%s {\n%s}", strings.Replace(key, "in", ":= range", 1),
					createPatternFunc(val.(map[string]interface{})))
			} else {
				panic(fmt.Sprintf("Unknown keywors at %s, %v", key, val))
			}
		}
	}
	return
}

func createIfStatement(statement map[string]interface{}) (condition, body string) {
	for key, val := range statement {
		return key, createPatternFunc(val.(map[string]interface{}))
	}
	return
}

func createItem(item map[string]interface{}) (code string) {
	code = fmt.Sprintf("leiogo.NewItem(leiogo.Dict%s)", eval(item))
	return
}

func createRequest(req map[string]interface{}) (code string) {
	code = "&leiogo.Request{"
	for key, val := range req {
		switch key {

		case "Meta":
			// There are two ways to set the Meta field.
			switch val.(type) {
			// The first one is to create a json object/
			case map[string]interface{}:
				code += fmt.Sprintf("Meta: leiogo.Dict%s, ", eval(val))
			// The second way is to create by a code string ($...$)
			case string:
				code += fmt.Sprintf("Meta: %s, ", eval(val))
			}

		default:
			code += fmt.Sprintf("%s: %v, ", key, eval(val))
		}
	}
	// If user doesn't provide the ParserName, we should always set it to 'parser'
	if _, ok := req["ParserName"]; !ok {
		code += "ParserName: \"parser\", "
	}
	// If user doesn't provide the Meta, we should create an empty one.
	if _, ok := req["Meta"]; !ok {
		code += "Meta: leiogo.Dict{}"
	}
	code += "}"
	return
}

func eval(val interface{}) interface{} {
	switch x := val.(type) {
	case string:
		if strings.HasPrefix(x, "$") {
			return x[1 : len(x)-1]
		} else {
			return "\"" + x + "\""
		}
	case map[string]interface{}:
		return evalDict(x)
	case []interface{}:
		return evalArray(x)
	default:
		return x
	}
}

func evalDict(dic map[string]interface{}) (code string) {
	code = "{"
	for key, val := range dic {
		code += fmt.Sprintf("\"%s\": %v, ", key, eval(val))
	}
	code += "}"
	return
}

func evalArray(a []interface{}) (code string) {
	code = "{"
	for _, val := range a {
		code += fmt.Sprintf("%v, ", eval(val))
	}
	code += "}"
	return
}
