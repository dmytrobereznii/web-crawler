package crawler

import (
	"context"
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
	Fetch(u *url.URL) (statusCode int, duration time.Duration, err error)
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
				err := c.store.UpdateStatus(crawlJob.ID, CrawlStatusInProgress)
				if err != nil {
					c.logger.Error().Err(err).Str("ID", crawlJob.ID.String()).Msg("failed to start processing")
					continue
				}

				statusCode, dur, err := c.fetcher.Fetch(crawlJob.URL)
				if err != nil {
					// TODO: how to make it shorter?
					c.logger.Error().Err(err).Str("ID", crawlJob.ID.String()).Dur("duration", dur).Int("code", statusCode).Msg("failed to fetch")
					continue
				}

				err = c.store.UpdateStatus(crawlJob.ID, CrawlStatusDone)
				if err != nil {
					c.logger.Error().Err(err).Str("ID", crawlJob.ID.String()).Dur("duration", dur).Int("code", statusCode).Msg("failed to update status")
					continue
				}

				c.logger.Info().Str("ID", crawlJob.ID.String()).Dur("duration", dur).Int("code", statusCode).Msg("fetched")
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
