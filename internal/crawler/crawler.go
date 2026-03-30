package crawler

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Crawler struct {
	logger       zerolog.Logger
	frontier     chan CrawlJob
	fetcher      fetcher
	workersCount int
	store        store
}

type CrawlJob struct {
	ID  uuid.UUID
	URL *url.URL
}

type fetcher interface {
	Fetch(u *url.URL) ([]*url.URL, time.Duration, error)
}

type store interface {
	UpdateStatus(id uuid.UUID, status CrawlStatus) error
}

func NewCrawler(logger zerolog.Logger, workersCount int, fetcher fetcher, store store) *Crawler {
	return &Crawler{
		logger:       logger,
		frontier:     make(chan CrawlJob),
		fetcher:      fetcher,
		workersCount: workersCount,
		store:        store,
	}
}

func (c *Crawler) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for i := 0; i < c.workersCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for crawlJob := range c.frontier {
				logWithID := c.logger.With().Str("ID", crawlJob.ID.String()).Logger()

				err := c.store.UpdateStatus(crawlJob.ID, CrawlStatusInProgress)
				if err != nil {
					logWithID.Error().Err(err).Msg("failed to start fetching")
					continue
				}

				links, dur, err := c.fetcher.Fetch(crawlJob.URL)
				if err != nil {
					logWithID.Error().Err(err).Dur("duration", dur).Msg("failed to fetch")
					continue
				}

				err = c.store.UpdateStatus(crawlJob.ID, CrawlStatusDone)
				if err != nil {
					logWithID.Error().Err(err).Dur("duration", dur).Msg("failed to update status")
					continue
				}

				logWithID.Info().Dur("duration", dur).Msg("fetched")

				fmt.Println(len(links))
			}
		}()
	}

	<-ctx.Done()
	close(c.frontier)
	wg.Wait()
}

func (c *Crawler) Submit(ctx context.Context, id uuid.UUID, u *url.URL) {
	select {
	case c.frontier <- CrawlJob{ID: id, URL: u}:
	case <-ctx.Done():
	}
}
