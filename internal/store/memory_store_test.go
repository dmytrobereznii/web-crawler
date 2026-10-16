package store

import (
	"context"
	"errors"
	"testing"
	"time"

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
			crawlStore := NewMemoryCrawlStore()

			if d.crawl.ID != uuid.Nil {
				err := crawlStore.Save(context.Background(), d.crawl)
				if err != nil {
					t.Fatal(err)
				}
			}

			res, err := crawlStore.Get(context.Background(), d.id)

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

func TestCrawlStore_UpdateResult(t *testing.T) {
	id := uuid.New()

	data := []struct {
		name         string
		saved        bool
		updateID     uuid.UUID
		update       bool
		limitReached bool
		want         bool
		err          error
	}{
		{"limit_reached_true_persisted", true, id, true, true, true, nil},
		{"limit_reached_false_persisted", true, id, true, false, false, nil},
		{"unknown_id_returns_ErrNotFound", true, uuid.New(), true, true, false, ErrNotFound},
		{"saved_crawl_reads_false", true, id, false, false, false, nil},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			ctx := context.Background()
			crawlStore := NewMemoryCrawlStore()

			if d.saved {
				if err := crawlStore.Save(ctx, crawler.Crawl{ID: id}); err != nil {
					t.Fatal(err)
				}
			}

			if d.update {
				err := crawlStore.UpdateResult(ctx, d.updateID, time.Second, 3, d.limitReached)
				if !errors.Is(err, d.err) {
					t.Fatalf("unexpected error: got %v, want %v", err, d.err)
				}
			}

			res, err := crawlStore.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if res.LimitReached != d.want {
				t.Errorf("unexpected LimitReached: got %v, want %v", res.LimitReached, d.want)
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
			crawlStore := NewMemoryCrawlStore()

			if d.expectedCrawl.ID != uuid.Nil {
				err = crawlStore.Save(context.Background(), d.expectedCrawl)
				if err != nil {
					t.Fatal(err)
				}
			}

			err = crawlStore.Save(context.Background(), crawler.Crawl{
				ID:     id,
				Status: crawler.CrawlStatusPending,
			})

			if d.err == nil {
				if err != nil {
					t.Errorf("unexpected error: got %v, want %v", err, d.err)
				}

				res, err := crawlStore.Get(context.Background(), d.id)
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
