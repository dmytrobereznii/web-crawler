package crawler

import (
	"context"
	"fmt"
	"net/url"
	"sync"
)

type Crawler struct {
	frontier     chan *url.URL
	workersCount int
}

func NewCrawler(workersCount int) *Crawler {
	return &Crawler{
		frontier:     make(chan *url.URL),
		workersCount: workersCount,
	}
}

func (c *Crawler) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for i := 0; i < c.workersCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for v := range c.frontier {
				fmt.Println(v)
			}
		}()
	}

	<-ctx.Done() //TODO: cancel after 5s
	close(c.frontier)
	wg.Wait()
}

func (c *Crawler) Submit(u *url.URL) {
	c.frontier <- u
}
