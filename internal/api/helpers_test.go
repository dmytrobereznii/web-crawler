package api_test

import (
	"context"
	"net/url"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/google/uuid"
)

type mockCrawler struct { // implements crawlSubmitter
	called bool
}

func (c *mockCrawler) Submit(ctx context.Context, id uuid.UUID, targetURL url.URL, seedURL url.URL) {
	c.called = true
}

type mockStore struct { // implements crawlStore
	crawl crawler.Crawl
	err   error
}

func (m *mockStore) Get(_ uuid.UUID) (crawler.Crawl, error) {
	return m.crawl, m.err
}

func (m *mockStore) Save(_ crawler.Crawl) error {
	return m.err
}
