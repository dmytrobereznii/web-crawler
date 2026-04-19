package crawler

import (
	"net/url"

	"github.com/google/uuid"
)

type Crawl struct {
	ID       uuid.UUID
	Status   CrawlStatus
	URL      *url.URL // TODO: no need for pointer
	Duration int64
	Visits   int64
}

type CrawlStatus string

const (
	CrawlStatusPending    CrawlStatus = "pending"
	CrawlStatusInProgress CrawlStatus = "in_progress"
	CrawlStatusDone       CrawlStatus = "done"
	CrawlStatusFailed     CrawlStatus = "failed"
)
