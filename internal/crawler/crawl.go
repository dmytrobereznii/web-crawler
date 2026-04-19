package crawler

import (
	"net/url"
	"time"

	"github.com/google/uuid"
)

type Crawl struct {
	ID       uuid.UUID
	Status   CrawlStatus
	URL      url.URL
	Duration time.Duration
	Visits   int64
}

type CrawlStatus string

const (
	CrawlStatusPending    CrawlStatus = "pending"
	CrawlStatusInProgress CrawlStatus = "in_progress"
	CrawlStatusDone       CrawlStatus = "done"
	CrawlStatusFailed     CrawlStatus = "failed"
)
