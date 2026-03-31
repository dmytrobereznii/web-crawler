package crawler

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
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
	visited      sync.Map
	results      sync.Map
}

type CrawlJob struct {
	ID      uuid.UUID
	URL     *url.URL
	SeedURL *url.URL
}

type CrawlResult struct {
	totalDuration atomic.Int64
	totalVisits   atomic.Int64
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
		frontier:     make(chan CrawlJob, 500),
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
				logWithIDAndURL := c.logger.With().Str("ID", crawlJob.ID.String()).Str("URL", crawlJob.URL.String()).Logger()

				links, dur, err := c.fetcher.Fetch(crawlJob.URL)
				if err != nil {
					logWithIDAndURL.Error().Err(err).Dur("duration", dur).Msg("failed to fetch")
					continue
				}

				logWithIDAndURL.Info().Dur("duration", dur).Msg("fetched")

				val, _ := c.results.LoadOrStore(crawlJob.ID, &CrawlResult{})
				crawlRes := val.(*CrawlResult)
				crawlRes.totalDuration.Add(dur.Milliseconds())
				crawlRes.totalVisits.Add(1)

				for _, link := range links {
					c.Submit(ctx, crawlJob.ID, link, crawlJob.SeedURL)
				}
			}
		}()
	}

	<-ctx.Done()
	close(c.frontier)
	wg.Wait()
}

func (c *Crawler) Submit(ctx context.Context, id uuid.UUID, targetURL *url.URL, seedURL *url.URL) {
	_, alreadyVisited := c.visited.LoadOrStore(targetURL.String(), struct{}{})
	if alreadyVisited {
		return
	}

	if !strings.HasPrefix(targetURL.String(), seedURL.String()) {
		return
	}

	if targetURL.String() == seedURL.String() {
		err := c.store.UpdateStatus(id, CrawlStatusInProgress)
		if err != nil {
			return
		}
	}

	select {
	case c.frontier <- CrawlJob{ID: id, URL: targetURL, SeedURL: seedURL}:
	case <-ctx.Done():
	}
}
