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
	trackers     sync.Map
}

type CrawlJob struct {
	ID      uuid.UUID
	URL     *url.URL
	SeedURL *url.URL
}

type CrawlTracker struct {
	totalDuration atomic.Int64
	totalVisits   atomic.Int64
	completeWG    *sync.WaitGroup
}

type fetcher interface {
	Fetch(ctx context.Context, u *url.URL) ([]*url.URL, time.Duration, error)
}

type store interface {
	UpdateStatus(id uuid.UUID, status CrawlStatus) error
	UpdateResult(id uuid.UUID, duration time.Duration, visits int64) error
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

			for {
				select {
				case crawlJob, ok := <-c.frontier:
					if !ok {
						return
					}

					val, _ := c.trackers.Load(crawlJob.ID)
					tracker := val.(*CrawlTracker)

					logWithIDAndURL := c.logger.With().Str("ID", crawlJob.ID.String()).Str("URL", crawlJob.URL.String()).Logger()

					links, dur, err := c.fetcher.Fetch(ctx, crawlJob.URL)
					if err != nil {
						logWithIDAndURL.Error().Err(err).Dur("duration", dur).Msg("failed to fetch")
						tracker.completeWG.Done()
						continue
					}

					logWithIDAndURL.Info().Dur("duration", dur).Msg("fetched")

					tracker.totalDuration.Add(dur.Nanoseconds())
					tracker.totalVisits.Add(1)

					for _, link := range links {
						c.Submit(ctx, crawlJob.ID, link, crawlJob.SeedURL)
					}

					tracker.completeWG.Done()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()
	close(c.frontier)
	for job := range c.frontier {
		val, _ := c.trackers.Load(job.ID)
		tracker := val.(*CrawlTracker)
		tracker.completeWG.Done()
	}
	c.logger.Debug().Msg("crawler finished")
}

func (c *Crawler) Submit(ctx context.Context, id uuid.UUID, targetURL *url.URL, seedURL *url.URL) {
	_, alreadyVisited := c.visited.LoadOrStore(targetURL.String(), struct{}{})
	if alreadyVisited {
		return
	}

	if !strings.HasPrefix(targetURL.String(), seedURL.String()) {
		return
	}

	val, _ := c.trackers.LoadOrStore(id, &CrawlTracker{completeWG: &sync.WaitGroup{}})
	tracker := val.(*CrawlTracker)
	tracker.completeWG.Add(1)

	if targetURL.String() == seedURL.String() {
		err := c.store.UpdateStatus(id, CrawlStatusInProgress)
		if err != nil {
			c.logger.Error().Err(err).Msg("failed to update status to in-progress")
			tracker.completeWG.Done()
			return
		}
		go func() {
			tracker.completeWG.Wait()
			err = c.store.UpdateStatus(id, CrawlStatusDone)
			if err != nil {
				c.logger.Error().Err(err).Msg("failed to update crawl status to done")
			}
			err = c.store.UpdateResult(id, time.Duration(tracker.totalDuration.Load()), tracker.totalVisits.Load())
			if err != nil {
				c.logger.Error().Err(err).Msg("failed to update crawl result")
			}
			c.logger.Debug().Msg("completion goroutine finished")
		}()
	}

	select {
	case c.frontier <- CrawlJob{ID: id, URL: targetURL, SeedURL: seedURL}:
	case <-ctx.Done():
		tracker.completeWG.Done()
	}
}
