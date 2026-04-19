package store

import (
	"errors"
	"time"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/google/uuid"
)

type MemoryCrawlStore struct {
	crawls map[uuid.UUID]crawler.Crawl
}

func NewMemoryCrawlStore() *MemoryCrawlStore {
	return &MemoryCrawlStore{
		crawls: make(map[uuid.UUID]crawler.Crawl),
	}
}

var (
	ErrNotFound      = errors.New("crawl not found")
	ErrAlreadyExists = errors.New("crawl already exists")
)

func (cs *MemoryCrawlStore) Get(id uuid.UUID) (crawler.Crawl, error) {
	c, ok := cs.crawls[id]

	if !ok {
		return crawler.Crawl{}, ErrNotFound
	}

	return c, nil
}

func (cs *MemoryCrawlStore) Save(crawl crawler.Crawl) error {
	_, ok := cs.crawls[crawl.ID]

	if ok {
		return ErrAlreadyExists
	}

	cs.crawls[crawl.ID] = crawl
	return nil
}

func (cs *MemoryCrawlStore) UpdateStatus(id uuid.UUID, status crawler.CrawlStatus) error {
	c, ok := cs.crawls[id]

	if !ok {
		return ErrNotFound
	}

	c.Status = status
	cs.crawls[id] = c
	return nil
}

func (cs *MemoryCrawlStore) UpdateResult(id uuid.UUID, duration time.Duration, visits int64) error {
	c, ok := cs.crawls[id]

	if !ok {
		return ErrNotFound
	}

	c.Duration = duration
	c.Visits = visits
	cs.crawls[id] = c
	return nil
}
