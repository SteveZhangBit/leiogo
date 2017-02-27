package crawler

import (
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/SteveZhangBit/leiogo/log"

	"github.com/SteveZhangBit/leiogo"
	"github.com/SteveZhangBit/leiogo/util"

	"time"
)

type ConcurrentCount struct {
	count int
	done  chan bool
}

func (c *ConcurrentCount) Add() {
	c.done <- true
}

func (c *ConcurrentCount) Done() {
	c.done <- false
}

func (c *ConcurrentCount) Wait() {
	for {
		if ok := <-c.done; ok {
			c.count++
		} else {
			c.count--
			if c.count <= 0 {
				break
			}
		}
	}
}

// The crawler will catch the interrupt signal from OS.
// The process won't stop immediately when user press ctrl+c, instead,
// it will wait for the running requests and items to complete,
// and refuse any further product.
type UserInterrupt struct {
	StatusInfo *StatusInfo
	Logger     log.Logger

	interrupt chan os.Signal
	closed    chan bool
}

func (u *UserInterrupt) Open(spider *leiogo.Spider) error {
	u.interrupt = make(chan os.Signal, 1)
	u.closed = make(chan bool)

	signal.Notify(u.interrupt, os.Interrupt)
	go func() {
		for {
			select {
			case <-u.interrupt:
				u.StatusInfo.Interrupt()
				u.Logger.Info(spider.Name, "Get user interrupt signal, waiting the running requests to complete")
			case <-u.closed:
				break
			}
		}
	}()
	return nil
}

func (u *UserInterrupt) Close(reason string, spider *leiogo.Spider) error {
	u.closed <- true
	return nil
}

// This struct is holded by the crawler to indicate the status of the spider.
// Since this would be changed by different goroutines, so it should be thread-safe.
// Use the Add... methods, and never change the field directly.
type StatusInfo struct {
	Logger log.Logger

	StartDate time.Time
	EndDate   time.Time

	// The reason why the spider is closed, it could be Jobs completed or User interrupted.
	Reason string

	// The url of pages which are processing.
	RunningPages map[string]struct{}

	// Number of pages yielded by the spider, including the droped ones.
	Pages int

	// Number of pages successfully downloaded by the downloader.
	Crawled int

	// Number of pages successfully reaching the parsers.
	Succeed int

	// All items yielded, including the droped ones.
	Items int

	// If user enable image download feature for the crawler, this field will show how many images have downloaded.
	Files int

	// This boolean indicates whether the crawler has been interrupted by user (ctrl+c).
	// The addRequest method will check this boolean when adding a new request.
	Interrupted bool

	mutex  sync.Mutex
	closed chan bool
}

func (s *StatusInfo) Open(spider *leiogo.Spider) error {
	s.closed = make(chan bool)
	ticker := time.NewTicker(60 * time.Second)

	s.StartDate = time.Now()
	s.Reason = "Jobs completed"

	go func() {
		for {
			select {
			case <-ticker.C:
				for _, line := range s.Report() {
					s.Logger.Info(spider.Name, line)
				}
			case <-s.closed:
				break
			}
		}
	}()

	return nil
}

func (s *StatusInfo) Close(reason string, spider *leiogo.Spider) error {
	s.EndDate = time.Now()
	s.closed <- true

	// Generate a final report
	s.Logger.Info(spider.Name, "%-10s - %s", "Start Date", s.StartDate.Format("2006-01-02 15:04:05"))
	s.Logger.Info(spider.Name, "%-10s - %s", "End Date", s.EndDate.Format("2006-01-02 15:04:05"))
	s.Logger.Info(spider.Name, "%-10s - %s", "Duration", util.FormatDuration(s.EndDate.Sub(s.StartDate)))
	s.Logger.Info(spider.Name, "%-10s - %d", "Pages", s.Pages)
	s.Logger.Info(spider.Name, "%-10s - %d", "Crawled", s.Crawled)
	s.Logger.Info(spider.Name, "%-10s - %d", "Succeed", s.Succeed)
	s.Logger.Info(spider.Name, "%-10s - %d", "Items", s.Items)
	s.Logger.Info(spider.Name, "%-10s - %d", "Files", s.Files)
	s.Logger.Info(spider.Name, "%-10s - %s", "Reason", s.Reason)

	return nil
}

func (s *StatusInfo) Report() []string {
	duration := time.Now().Sub(s.StartDate)
	return []string{
		fmt.Sprintf("%-10s - %s", "Duration", util.FormatDuration(duration)),
		fmt.Sprintf("%-10s - %d (%.1f per minute)", "Pages", s.Pages, float64(s.Pages)/duration.Minutes()),
		fmt.Sprintf("%-10s - %d (%.1f per minute)", "Crawled", s.Crawled, float64(s.Crawled)/duration.Minutes()),
		fmt.Sprintf("%-10s - %d (%.1f per minute)", "Succeed", s.Succeed, float64(s.Succeed)/duration.Minutes()),
		fmt.Sprintf("%-10s - %d (%.1f per minute)", "Items", s.Items, float64(s.Items)/duration.Minutes()),
		fmt.Sprintf("%-10s - %d (%.1f per minute)", "Files", s.Files, float64(s.Files)/duration.Minutes()),
	}
}

func (s *StatusInfo) Interrupt() {
	s.Interrupted = true
	s.Reason = "User interrupted"
}

func (s *StatusInfo) IsInterrupt() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.Interrupted
}

func (s *StatusInfo) AddPage() {
	s.mutex.Lock()
	s.Pages++
	s.mutex.Unlock()
}

func (s *StatusInfo) AddRunningPage(req *leiogo.Request) {
	s.mutex.Lock()
	if s.RunningPages == nil {
		s.RunningPages = make(map[string]struct{})
	}
	s.RunningPages[req.URL] = struct{}{}
	s.mutex.Unlock()
}

func (s *StatusInfo) AddCrawled() {
	s.mutex.Lock()
	s.Crawled++
	s.mutex.Unlock()
}

func (s *StatusInfo) AddFiles() {
	s.mutex.Lock()
	s.Files++
	s.mutex.Unlock()
}

func (s *StatusInfo) AddSucceed(req *leiogo.Request) {
	s.mutex.Lock()
	s.Succeed++
	delete(s.RunningPages, req.URL)
	s.mutex.Unlock()
}

func (s *StatusInfo) AddItem() {
	s.mutex.Lock()
	s.Items++
	s.mutex.Unlock()
}
