package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/dmytrobereznii/web-crawler/internal/api"
	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/dmytrobereznii/web-crawler/internal/store"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type getCrawlResponse struct {
	ID     string              `json:"id"`
	Status crawler.CrawlStatus `json:"status"`
}

type mockCrawler struct{} // implements crawlSubmitter

func (c *mockCrawler) Submit(ctx context.Context, id uuid.UUID, targetURL *url.URL, seedURL *url.URL) {
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

func TestHandler_GetCrawl(t *testing.T) {
	id := uuid.New()

	cases := []struct {
		name  string
		id    string
		crawl crawler.Crawl
		err   error
		code  int
	}{
		{"invalid_id_returns_422", "123", crawler.Crawl{}, nil, http.StatusUnprocessableEntity},
		{"incorrect_id_returns_404", id.String(), crawler.Crawl{}, store.ErrNotFound, http.StatusNotFound},
		{"correct_id_returns_crawl", id.String(), crawler.Crawl{ID: id}, nil, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mockStore{crawl: tc.crawl, err: tc.err}
			c := &mockCrawler{}
			handler := api.NewHandler(
				zerolog.Nop(),
				s,
				c,
			)

			req := httptest.NewRequest(http.MethodGet, "/crawls/"+tc.id, nil)
			req.SetPathValue("id", tc.id)

			rec := httptest.NewRecorder()
			handler.GetCrawl(rec, req)

			if rec.Code != tc.code {
				t.Errorf("unexpected code: got %d, want %d", rec.Code, tc.code)
			}

			if tc.code == http.StatusOK {
				var got getCrawlResponse
				err := json.NewDecoder(rec.Body).Decode(&got)

				if err != nil {
					t.Errorf("unexpected error when decoding response: %v", err)
				}
				if got.ID != tc.id {
					t.Errorf("unexpected id: got %s, want %s", got.ID, tc.id)
				}
				if got.Status != tc.crawl.Status {
					t.Errorf("unexpected status: got %s, want %s", got.Status, tc.crawl.Status)
				}
			}
		})
	}
}
