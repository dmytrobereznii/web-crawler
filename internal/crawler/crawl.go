package crawler

import (
	"github.com/google/uuid"
)

type Crawl struct {
	ID     uuid.UUID
	Status CrawlStatus
	URL    string
}

type CrawlStatus string

const (
	CrawlStatusPending    CrawlStatus = "pending"
	CrawlStatusInProgress CrawlStatus = "in_progress"
	CrawlStatusDone       CrawlStatus = "done"
	CrawlStatusFailed     CrawlStatus = "failed"
)
