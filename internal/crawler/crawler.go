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
	maxPages     int
	store        store
	visited      sync.Map
	trackers     sync.Map
	ctx          context.Context
}

type CrawlJob struct {
	ID      uuid.UUID
	URL     url.URL
	SeedURL url.URL
}

type CrawlTracker struct {
	totalDuration atomic.Int64
	totalVisits   atomic.Int64
	reserved      atomic.Int64 // fetch slots claimed; totalVisits <= reserved <= maxPages
	limitReached  atomic.Bool
	completeWG    *sync.WaitGroup
}

// tryReserve claims one fetch slot if fewer than limit are taken.
// Compare-and-swap loop: if another worker wins the race, CompareAndSwap
// fails, we reload and re-check, so the count can never pass limit.
func (t *CrawlTracker) tryReserve(limit int64) bool {
	for cur := t.reserved.Load(); cur < limit; cur = t.reserved.Load() {
		if t.reserved.CompareAndSwap(cur, cur+1) {
			return true
		}
	}
	return false
}

// release returns a slot after a failed fetch (failed fetches do not count).
func (t *CrawlTracker) release() {
	t.reserved.Add(-1)
}

type fetcher interface {
	Fetch(ctx context.Context, u url.URL) ([]url.URL, time.Duration, error)
}

type store interface {
	UpdateStatus(context.Context, uuid.UUID, CrawlStatus) error
	UpdateResult(ctx context.Context, id uuid.UUID, duration time.Duration, visits int64, limitReached bool) error
}

// NewCrawler builds a crawler. maxPages caps fetched pages per crawl; 0 means unlimited.
func NewCrawler(logger zerolog.Logger, workersCount int, maxPages int, fetcher fetcher, store store) *Crawler {
	return &Crawler{
		logger:       logger,
		frontier:     make(chan CrawlJob, 500),
		fetcher:      fetcher,
		workersCount: workersCount,
		maxPages:     maxPages,
		store:        store,
		ctx:          context.Background(),
	}
}

func (c *Crawler) limited() bool {
	return c.maxPages > 0
}

// markLimitReached records that a URL went unfetched because of the cap.
// CompareAndSwap(false, true) succeeds for exactly one caller, so only
// the winner logs and the line is emitted once per crawl without a mutex.
func (c *Crawler) markLimitReached(id uuid.UUID, tracker *CrawlTracker) {
	if tracker.limitReached.CompareAndSwap(false, true) {
		c.logger.Info().Str("ID", id.String()).Int("max_pages", c.maxPages).Msg("max pages per crawl reached")
	}
}

func (c *Crawler) Run(ctx context.Context) {
	c.ctx = ctx
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

					if c.limited() && !tracker.tryReserve(int64(c.maxPages)) {
						c.markLimitReached(crawlJob.ID, tracker)
						tracker.completeWG.Done()
						continue
					}

					links, dur, err := c.fetcher.Fetch(ctx, crawlJob.URL)
					if err != nil {
						logWithIDAndURL.Error().Err(err).Dur("duration", dur).Msg("failed to fetch")
						if c.limited() {
							tracker.release()
						}
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

func (c *Crawler) Submit(ctx context.Context, id uuid.UUID, targetURL url.URL, seedURL url.URL) {
	_, alreadyVisited := c.visited.LoadOrStore(targetURL.String(), struct{}{})
	if alreadyVisited {
		return
	}

	if !strings.HasPrefix(targetURL.String(), seedURL.String()) {
		return
	}

	val, _ := c.trackers.LoadOrStore(id, &CrawlTracker{completeWG: &sync.WaitGroup{}})
	tracker := val.(*CrawlTracker)

	// Drop before completeWG.Add: a dropped URL never joins the WaitGroup, so
	// there is no Done to pair. It sits after dedup and the prefix filter so
	// only in-scope, unvisited URLs count toward limit_reached.
	if c.limited() && tracker.totalVisits.Load() >= int64(c.maxPages) {
		c.markLimitReached(id, tracker)
		return
	}

	tracker.completeWG.Add(1)

	if targetURL.String() == seedURL.String() {
		err := c.store.UpdateStatus(ctx, id, CrawlStatusInProgress)
		if err != nil {
			c.logger.Error().Err(err).Msg("failed to update status to in-progress")
			tracker.completeWG.Done()
			return
		}
		go func() {
			tracker.completeWG.Wait()
			err = c.store.UpdateStatus(c.ctx, id, CrawlStatusDone)
			if err != nil {
				c.logger.Error().Err(err).Msg("failed to update crawl status to done")
			}
			err = c.store.UpdateResult(c.ctx, id, time.Duration(tracker.totalDuration.Load()), tracker.totalVisits.Load(), tracker.limitReached.Load())
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
