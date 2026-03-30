package store

import (
	"errors"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/google/uuid"
)

type CrawlStore struct {
	crawls map[uuid.UUID]crawler.Crawl
}

func NewCrawlStore() *CrawlStore {
	return &CrawlStore{
		crawls: make(map[uuid.UUID]crawler.Crawl),
	}
}

var (
	ErrNotFound      = errors.New("crawl not found")
	ErrAlreadyExists = errors.New("crawl already exists")
)

func (cs *CrawlStore) Get(id uuid.UUID) (crawler.Crawl, error) {
	c, ok := cs.crawls[id]

	if !ok {
		return crawler.Crawl{}, ErrNotFound
	}

	return c, nil
}

func (cs *CrawlStore) Save(crawl crawler.Crawl) error {
	_, ok := cs.crawls[crawl.ID]

	if ok {
		return ErrAlreadyExists
	}

	cs.crawls[crawl.ID] = crawl
	return nil
}

func (cs *CrawlStore) UpdateStatus(id uuid.UUID, status crawler.CrawlStatus) error {
	c, ok := cs.crawls[id]

	if !ok {
		return ErrNotFound
	}

	c.Status = status
	cs.crawls[id] = c
	return nil
}
