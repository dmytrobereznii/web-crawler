package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	// Pointer so a missing field decodes as nil, distinct from false.
	LimitReached *bool `json:"limit_reached"`
}

func TestHandler_GetCrawl(t *testing.T) {
	id := uuid.New()
	limitTrue, limitFalse := true, false

	cases := []struct {
		name  string
		id    string
		crawl crawler.Crawl
		err   error
		code  int
		// wantLimit, when non-nil, is the expected limit_reached value; the
		// field must be present in the response.
		wantLimit *bool
	}{
		{"invalid_id_returns_422", "123", crawler.Crawl{}, nil, http.StatusUnprocessableEntity, nil},
		{"incorrect_id_returns_404", id.String(), crawler.Crawl{}, store.ErrNotFound, http.StatusNotFound, nil},
		{"correct_id_returns_crawl", id.String(), crawler.Crawl{ID: id}, nil, http.StatusOK, nil},
		{"limit_reached_true_returned", id.String(), crawler.Crawl{ID: id, LimitReached: true}, nil, http.StatusOK, &limitTrue},
		{"limit_reached_false_always_present", id.String(), crawler.Crawl{ID: id, LimitReached: false}, nil, http.StatusOK, &limitFalse},
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
				if tc.wantLimit != nil {
					if got.LimitReached == nil {
						t.Errorf("limit_reached missing from response, want %v", *tc.wantLimit)
					} else if *got.LimitReached != *tc.wantLimit {
						t.Errorf("unexpected limit_reached: got %v, want %v", *got.LimitReached, *tc.wantLimit)
					}
				}
			}
		})
	}
}
