package crawler

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type mockStore struct { // implements crawlStore
	status CrawlStatus
	done   chan struct{}
}

func (s *mockStore) UpdateStatus(id uuid.UUID, status CrawlStatus) error {
	s.status = status
	return nil
}

func (s *mockStore) UpdateResult(id uuid.UUID, duration int64, visits int64) error {
	close(s.done)
	return nil
}

type mockFetcher struct { // implements fetcher
	urlsToReturn map[string][]string
	visits       int
}

func (f *mockFetcher) Fetch(ctx context.Context, u *url.URL) ([]*url.URL, time.Duration, error) {
	f.visits++
	var parsedURLs []*url.URL
	urlsToReturn := f.urlsToReturn[u.String()]
	for _, v := range urlsToReturn {
		parsedURL, err := url.Parse(v)
		if err != nil {
			panic(err)
		}
		parsedURLs = append(parsedURLs, parsedURL)
	}
	return parsedURLs, time.Duration(123), nil
}

func TestCrawler_Submit_Deduplication(t *testing.T) {
	cases := []struct {
		name       string
		targetURLs []string
		seedURL    string
		onFrontier int
	}{
		{
			"same_urls_get_deduplicated",
			[]string{"https://github.com/joho/godotenv/tree/main/cmd/godotenv", "https://github.com/joho/godotenv/tree/main/cmd/godotenv"},
			"https://github.com/joho/godotenv/tree/main",
			1,
		},
		{
			"different_urls_get_submitted",
			[]string{"https://github.com/joho/godotenv/tree/main/cmd/godotenv", "https://github.com/joho/godotenv/tree/main/fixtures"},
			"https://github.com/joho/godotenv/tree/main",
			2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mockStore{}
			f := &mockFetcher{}
			c := NewCrawler(
				zerolog.Nop(),
				1,
				f,
				s,
			)

			targetURLs := make([]*url.URL, 0, len(tc.targetURLs))
			for _, u := range tc.targetURLs {
				parsedURL, err := url.Parse(u)
				if err != nil {
					t.Fatal(err)
				}

				targetURLs = append(targetURLs, parsedURL)
			}

			seedURL, err := url.Parse(tc.seedURL)
			if err != nil {
				t.Fatal(err)
			}

			for _, u := range targetURLs {
				c.Submit(context.Background(), uuid.New(), u, seedURL)
			}

			if tc.onFrontier != len(c.frontier) {
				t.Errorf("onFrontier = %d, want %d", len(c.frontier), tc.onFrontier)
			}
		})
	}
}

func TestCrawler_Submit_SeedFiltering(t *testing.T) {
	cases := []struct {
		name       string
		targetURL  string
		seedURL    string
		onFrontier int
	}{
		{
			"urls_different_from_seed_get_rejected",
			"https://github.com/joho/godotenv/blob/main/go.mod",
			"https://github.com/joho/godotenv/tree/main",
			0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mockStore{}
			f := &mockFetcher{}
			c := NewCrawler(
				zerolog.Nop(),
				1,
				f,
				s,
			)

			targetURL, err := url.Parse(tc.targetURL)
			if err != nil {
				t.Fatal(err)
			}

			seedURL, err := url.Parse(tc.seedURL)
			if err != nil {
				t.Fatal(err)
			}

			c.Submit(context.Background(), uuid.New(), targetURL, seedURL)

			if tc.onFrontier != len(c.frontier) {
				t.Errorf("onFrontier = %d, want %d", len(c.frontier), tc.onFrontier)
			}
		})
	}
}

func TestCrawler_Submit_UpdatesStatus(t *testing.T) {
	cases := []struct {
		name           string
		targetURL      string
		seedURL        string
		expectedStatus CrawlStatus
	}{
		{
			"status_updated_on_first_submit",
			"https://github.com/joho/godotenv/tree/main",
			"https://github.com/joho/godotenv/tree/main",
			CrawlStatusInProgress,
		},
		{
			"status__not_updated_on_n_submit",
			"https://github.com/joho/godotenv/tree/main/cmd/godotenv",
			"https://github.com/joho/godotenv/tree/main",
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mockStore{}
			f := &mockFetcher{}
			c := NewCrawler(
				zerolog.Nop(),
				1,
				f,
				s,
			)

			targetURL, err := url.Parse(tc.targetURL)
			if err != nil {
				t.Fatal(err)
			}

			seedURL, err := url.Parse(tc.seedURL)
			if err != nil {
				t.Fatal(err)
			}

			c.Submit(context.Background(), uuid.New(), targetURL, seedURL)

			if s.status != tc.expectedStatus {
				t.Errorf("s.status = %s, want %s", s.status, tc.expectedStatus)
			}
		})
	}
}

func TestCrawler_Run_JobsProcessed(t *testing.T) {
	cases := []struct {
		name         string
		seedURL      string
		urlsToReturn map[string][]string
		visits       int
	}{
		{
			"single_jobs_processed",
			"https://github.com/joho/godotenv/tree/main",
			map[string][]string{"https://github.com/joho/godotenv/tree/main": {}},
			1,
		},
		{
			"multiple_jobs_processed",
			"https://github.com/joho/godotenv/tree/main",
			map[string][]string{"https://github.com/joho/godotenv/tree/main": {"https://github.com/joho/godotenv/tree/main/cmd/godotenv"}},
			2,
		},
		{
			"doesnt_visit_another_url",
			"https://github.com/joho/godotenv/tree/main",
			map[string][]string{"https://github.com/joho/godotenv/tree/main": {"https://github.com/joho/godotenv/blob/main/cmd/godotenv"}},
			1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mockStore{done: make(chan struct{})}
			f := &mockFetcher{urlsToReturn: tc.urlsToReturn}
			c := NewCrawler(
				zerolog.Nop(),
				1,
				f,
				s,
			)
			go c.Run(context.Background())

			targetURL, err := url.Parse(tc.seedURL)
			if err != nil {
				t.Fatal(err)
			}

			seedURL, err := url.Parse(tc.seedURL)
			if err != nil {
				t.Fatal(err)
			}

			c.Submit(context.Background(), uuid.New(), targetURL, seedURL)

			select {
			case <-s.done:
				if s.status != CrawlStatusDone {
					t.Errorf("s.status = %s, want %s", s.status, CrawlStatusDone)
				}
				if len(c.frontier) != 0 {
					t.Errorf("frontier must be empty, got %d", len(c.frontier))
				}
				if tc.visits != f.visits {
					t.Errorf("f.visits = %d, want %d", f.visits, tc.visits)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for jobs to complete")
			}
		})
	}
}

func TestCrawler_Run_RespectsCancellation(t *testing.T) {
	cases := []struct {
		name string
	}{
		{
			"run_returns_on_context_cance",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mockStore{done: make(chan struct{})}
			f := &mockFetcher{}
			c := NewCrawler(
				zerolog.Nop(),
				1,
				f,
				s,
			)

			ctx, cancelFunc := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				c.Run(ctx)
				close(done)
			}()
			cancelFunc()

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Errorf("timed out waiting for jobs to complete")
			}
		})
	}
}
