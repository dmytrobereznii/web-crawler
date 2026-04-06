package store

import (
	"errors"
	"testing"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/google/uuid"
)

func TestCrawlStore_Get(t *testing.T) {
	id := uuid.New()

	data := []struct {
		name  string
		id    uuid.UUID
		crawl crawler.Crawl
		err   error
	}{
		{"empty_store_returns_ErrNotFound", id, crawler.Crawl{}, ErrNotFound},
		{"correct_id_returns_valid_result", id, crawler.Crawl{ID: id}, nil},
		{"incorrect_id_returns_ErrNotFound", id, crawler.Crawl{ID: uuid.New()}, ErrNotFound},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			crawlStore := NewCrawlStore()

			if d.crawl.ID != uuid.Nil {
				err := crawlStore.Save(d.crawl)
				if err != nil {
					t.Fatal(err)
				}
			}

			res, err := crawlStore.Get(d.id)

			if d.err == nil {
				if err != nil {
					t.Errorf("unexpected error: got %v, want %v", err, d.err)
				}
				if res.ID != d.crawl.ID {
					t.Errorf("unexpected found: got %v, want %v", res, d.crawl)
				}
			}

			if d.err != nil {
				if !errors.Is(err, d.err) || err == nil {
					t.Errorf("unexpected error: got %v; want %v", err, d.err)
				}
			}
		})
	}
}

func TestCrawlStore_Save(t *testing.T) {
	id := uuid.New()

	data := []struct {
		name          string
		id            uuid.UUID
		expectedCrawl crawler.Crawl
		err           error
	}{
		{"new_id_successful", id, crawler.Crawl{}, nil},
		{"existing_id_returns_ErrAlreadyExists", id, crawler.Crawl{ID: id}, ErrAlreadyExists},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			var err error
			crawlStore := NewCrawlStore()

			if d.expectedCrawl.ID != uuid.Nil {
				err = crawlStore.Save(d.expectedCrawl)
				if err != nil {
					t.Fatal(err)
				}
			}

			err = crawlStore.Save(crawler.Crawl{
				ID:     id,
				Status: crawler.CrawlStatusPending,
			})

			if d.err == nil {
				if err != nil {
					t.Errorf("unexpected error: got %v, want %v", err, d.err)
				}

				res, err := crawlStore.Get(d.id)
				if err != nil {
					t.Errorf("unexpected error: got %v, want %v", err, d.err)
				}
				if res.ID != d.id {
					t.Errorf("unexpected result: got %v, want %v", res, d.expectedCrawl)
				}
			}

			if d.err != nil {
				if !errors.Is(err, d.err) {
					t.Errorf("unexpected error: got %v, want %v", err, d.err)
				}
			}
		})
	}
}
