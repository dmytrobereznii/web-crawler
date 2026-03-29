package crawler

import (
	"context"
	"net/url"
	"sync"

	"github.com/rs/zerolog"
)

type Crawler struct {
	logger       zerolog.Logger
	frontier     chan *url.URL
	fetcher      fetcher
	workersCount int
}

type fetcher interface {
	Fetch(u *url.URL) (int, error)
}

func NewCrawler(logger zerolog.Logger, workersCount int, fetcher fetcher) *Crawler {
	return &Crawler{
		logger:       logger,
		frontier:     make(chan *url.URL),
		fetcher:      fetcher,
		workersCount: workersCount,
	}
}

func (c *Crawler) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for i := 0; i < c.workersCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for u := range c.frontier {
				sc, err := c.fetcher.Fetch(u)
				if err != nil {
					c.logger.Error().Err(err).Str("url", u.String()).Int("code", sc).Msg("failed to fetch")
					continue
				}
				c.logger.Info().Str("url", u.String()).Int("code", sc).Msg("fetched")
			}
		}()
	}

	<-ctx.Done()
	close(c.frontier)
	wg.Wait()
}

func (c *Crawler) Submit(ctx context.Context, u *url.URL) {
	select {
	case c.frontier <- u:
	case <-ctx.Done():
	}
}
