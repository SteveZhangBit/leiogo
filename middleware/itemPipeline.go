package middleware

import (
	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/util"
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

// FilePipeline is simple pipeline to download static files, usually images.
// Since it is divided into two part, a pipeline and spider middleware,
// so we have to add these two parts to the crawler to make it available,
// or simply call AddImageDownloadSupport from the builder (See more in the crawler package).
type FilePipeline struct {
	Base

	// DirPath defines the directory where we want to store the downloaded files.
	// Both relative and absolute path are viable.
	// And there's no need to create the directory first, the pipeline will create the path if needed.
	DirPath string

	Yielder
}

func (p *FilePipeline) Open(spider *leiogo.Spider) error {
	p.Logger.Debug(spider.Name, "Init success with file directory: %s", p.DirPath)
	return nil
}

// Because file pipeline is an item pipeline, so we can just yield a special item with the target file information.
// Add fileurls (required) and filepath (optional) to the items, and the pipeline will catch such items,
// create new download requests for those urls.
func (p *FilePipeline) Process(item *leiogo.Item, spider *leiogo.Spider) error {
	// We have to first make sure that the item has fileurls attribute,
	// only such type of item will be treated as a file download item.
	if _, ok := item.Data["fileurls"]; !ok {
		return nil
	}

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
	for i, url := range item.Data["fileurls"].([]string) {

		// First to get the extension of the file to keep the filetype.
		// We offer two ways:
		// the first is using the extension in the url string, usually the last few words.
		// the second way is to add exts attribute to the item.
		var ext string
		if exts, ok := item.Data["exts"].([]string); !ok {
			ext = url[strings.LastIndex(url, "."):]
		} else {
			ext = exts[i]
		}

		// We won't use the original file name, instead we create a hashed name from its url.
		// We are using MD5 here.
		filepath := path.Join(subpath, util.MD5Hash(url)+ext)

		// Somtimes we will run the spider for several times, and there's no need to download
		// the files which are already exists, therefore we will first check the existance of the file.
		if info, err := os.Stat(filepath); os.IsNotExist(err) || info.Size() < 512 {

			// We might directely download the file here, but that's not a good idea.
			// We still want to take advantage of our previous work, like delay, offsite,
			// so we decide to yield a new request here, and add type and filepath information in the meta.
			// The Downloader will catch such requests and store the file to the
			// target path. See DefaultDownloader for more information.
			fileRequest := leiogo.NewRequest(url)
			fileRequest.Meta["__type__"] = "file"
			fileRequest.Meta["__filepath__"] = filepath

			if err := p.NewRequest(fileRequest, nil, spider); err != nil {
				p.Logger.Error(spider.Name, "Add file request error %s", err.Error())
			}
		}
	}
	return nil
}
