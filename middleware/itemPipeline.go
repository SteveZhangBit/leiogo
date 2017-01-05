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

// Return this type of error when we want to drop an item.
// This is similar to DropTaskError.
type DropItemError struct {
	Message string
}

func (err *DropItemError) Error() string {
	return err.Message
}

// ImagePipeline is simple pipeline to download static files, usually images.
// Since it is divided into two part, a pipeline and spider middleware,
// so we have to add these two parts to the crawler to make it available,
// or simply call AddImageDownloadSupport from the builder (See more in the crawler package).
type ImagePipeline struct {
	Base

	// DirPath defines the directory where we want to store the downloaded files.
	// Both relative and absolute path are viable.
	// And there's no need to create the directory first, the pipeline will create the path if needed.
	DirPath string

	Yielder
}

func (p *ImagePipeline) Open(spider *leiogo.Spider) error {
	p.Logger.Debug(spider.Name, "Init success with file directory: %s", p.DirPath)
	return nil
}

// Because image pipeline is an item pipeline, so we can just yield a special item with the target file information.
// Add fileurls (required) and filepath (optional) to the items, and the pipeline will catch such items,
// create new download requests for those urls.
func (p *ImagePipeline) Process(item *leiogo.Item, spider *leiogo.Spider) error {
	subpath := p.DirPath

	// We allowed the files to store in a sub path under the DirPath.
	// In some cases, we run a spider project to download images,
	// and each image may belong to a different thumbnail. To achieve this,
	// simply add filepath attribute to the item, it will create a new directory under the DirPath.
	if filepath, ok := item.Data["filepath"].(string); ok {
		subpath = path.Join(p.DirPath, filepath)
	}

	// Create the sub directory, in the same time we will also create the parent directory if needed.
	if err := os.MkdirAll(subpath, os.ModeDir); err != nil {
		p.Logger.Error(spider.Name, "Create directory failed, %s", err.Error())
	}

	// Traverse all the urls in the fileurls.
	for _, url := range item.Data["fileurls"].([]string) {

		// First to get the extension of the file to keep the filetype.
		ext := url[strings.LastIndex(url, "."):]

		// We won't use the original file name, instead we create a hashed name from its url.
		// We are using MD5 here.
		filepath := path.Join(subpath, hashURL(url)+ext)

		// Somtimes we will run the spider for several times, and there's no need to download
		// the files which are already exists, therefore we will first check the existance of the file.
		if info, err := os.Stat(filepath); os.IsNotExist(err) || info.Size() < 512 {

			// We might directely download the images here, but that's not a good idea.
			// We still want to take advantage of our previous work, like delay, offsite,
			// so we decide to yield a new request here, and add type and filepath information in the meta.
			// The SaveImageMiddleware will catch such requests and store the file to the
			// target path. See SaveImageMiddleware for more information.
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

// SaveimageMiddleware is a spider middlware to help download files yielded from the image pipeline.
type SaveImageMiddleware struct {
	BaseMiddleware
}

func (m *SaveImageMiddleware) ProcessResponse(res *leiogo.Response, req *leiogo.Request, spider *leiogo.Spider) error {
	typeName, typeOk := res.Meta["type"].(string)
	filepath, pathOK := res.Meta["filepath"].(string)

	// We set some special fields to the meta in order to identify these requests.
	if typeOk && pathOK && typeName == "file" {
		m.Logger.Info(spider.Name, "Saving %s to %s", req.URL, filepath)

		// Simply create the file and write the response to it.
		// Drop this request after download complete, because there's no need for this
		// request to go further.
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
