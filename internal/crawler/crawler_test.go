package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// crawlRecord is what mockStore remembers per crawl.
type crawlRecord struct {
	status       CrawlStatus
	visits       int64
	limitReached bool
}

// mockStore is safe for concurrent use. status mirrors the last status written
// for any crawl; read it only after receiving from done (or from a single goroutine).
type mockStore struct { // implements crawlStore
	mu     sync.Mutex
	status CrawlStatus
	crawls map[uuid.UUID]*crawlRecord
	// done receives the crawl ID from UpdateResult. It must be buffered.
	done chan uuid.UUID
}

// recordLocked returns the record for id, creating it. Caller holds s.mu.
func (s *mockStore) recordLocked(id uuid.UUID) *crawlRecord {
	if s.crawls == nil {
		s.crawls = make(map[uuid.UUID]*crawlRecord)
	}
	r, ok := s.crawls[id]
	if !ok {
		r = &crawlRecord{}
		s.crawls[id] = r
	}
	return r
}

func (s *mockStore) record(id uuid.UUID) crawlRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.recordLocked(id)
}

func (s *mockStore) UpdateStatus(_ context.Context, id uuid.UUID, status CrawlStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	s.recordLocked(id).status = status
	return nil
}

func (s *mockStore) UpdateResult(_ context.Context, id uuid.UUID, duration time.Duration, visits int64, limitReached bool) error {
	s.mu.Lock()
	r := s.recordLocked(id)
	r.visits = visits
	r.limitReached = limitReached
	s.mu.Unlock()
	if s.done != nil {
		s.done <- id
	}
	return nil
}

// mockFetcher is safe for concurrent use. visits counts every Fetch call,
// including failed ones. Read it only after the crawl is done.
type mockFetcher struct { // implements fetcher
	urlsToReturn map[string][]string
	failURLs     map[string]bool
	delay        time.Duration

	mu     sync.Mutex
	visits int
}

func (f *mockFetcher) fetchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.visits
}

func (f *mockFetcher) Fetch(ctx context.Context, u url.URL) ([]url.URL, time.Duration, error) {
	f.mu.Lock()
	f.visits++
	f.mu.Unlock()
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if f.failURLs[u.String()] {
		return nil, 0, errors.New("mock fetch failure")
	}
	var parsedURLs []url.URL
	urlsToReturn := f.urlsToReturn[u.String()]
	for _, v := range urlsToReturn {
		parsedURL, err := url.Parse(v)
		if err != nil {
			panic(err)
		}
		parsedURLs = append(parsedURLs, *parsedURL)
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
				0,
				f,
				s,
			)

			targetURLs := make([]url.URL, 0, len(tc.targetURLs))
			for _, u := range tc.targetURLs {
				parsedURL, err := url.Parse(u)
				if err != nil {
					t.Fatal(err)
				}

				targetURLs = append(targetURLs, *parsedURL)
			}

			seedURL, err := url.Parse(tc.seedURL)
			if err != nil {
				t.Fatal(err)
			}

			for _, u := range targetURLs {
				c.Submit(context.Background(), uuid.New(), u, *seedURL)
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
				0,
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

			c.Submit(context.Background(), uuid.New(), *targetURL, *seedURL)

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
				0,
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

			c.Submit(context.Background(), uuid.New(), *targetURL, *seedURL)

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
			s := &mockStore{done: make(chan uuid.UUID, 4)}
			f := &mockFetcher{urlsToReturn: tc.urlsToReturn}
			c := NewCrawler(
				zerolog.Nop(),
				1,
				0,
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

			c.Submit(context.Background(), uuid.New(), *targetURL, *seedURL)

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
			s := &mockStore{done: make(chan uuid.UUID, 4)}
			f := &mockFetcher{}
			c := NewCrawler(
				zerolog.Nop(),
				1,
				0,
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

// pagesUnder builds a site: the seed links to n-1 children under it.
func pagesUnder(seed string, n int) map[string][]string {
	children := make([]string, 0, n-1)
	for i := 1; i < n; i++ {
		children = append(children, fmt.Sprintf("%s/p%d", seed, i))
	}
	return map[string][]string{seed: children}
}

// runCrawler starts c.Run and returns a stop func that cancels it and waits for it to return.
func runCrawler(t *testing.T, c *Crawler) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	exited := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(exited)
	}()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
				t.Error("timed out waiting for Run to return")
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func waitCrawlDone(t *testing.T, s *mockStore) uuid.UUID {
	t.Helper()
	select {
	case id := <-s.done:
		return id
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for crawl to complete")
		return uuid.Nil
	}
}

func mustParse(t *testing.T, raw string) url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return *u
}

func TestCrawler_Run_MaxPages(t *testing.T) {
	const seed = "https://example.com/site"

	cases := []struct {
		name         string
		workers      int
		maxPages     int
		pages        map[string][]string
		failURLs     map[string]bool
		delay        time.Duration
		wantVisits   int64
		wantFetches  int
		wantLimitHit bool
	}{
		{
			name: "unlimited_fetches_every_page", workers: 2, maxPages: 0,
			pages: pagesUnder(seed, 6), wantVisits: 6, wantFetches: 6, wantLimitHit: false,
		},
		{
			name: "cap_holds_with_concurrent_workers", workers: 4, maxPages: 5,
			pages: pagesUnder(seed, 21), delay: 5 * time.Millisecond,
			wantVisits: 5, wantFetches: 5, wantLimitHit: true,
		},
		{
			name: "limit_one_fetches_only_seed", workers: 2, maxPages: 1,
			pages: pagesUnder(seed, 4), wantVisits: 1, wantFetches: 1, wantLimitHit: true,
		},
		{
			name: "exact_page_count_reports_false", workers: 2, maxPages: 3,
			pages: pagesUnder(seed, 3), wantVisits: 3, wantFetches: 3, wantLimitHit: false,
		},
		{
			name: "fewer_pages_than_limit_reports_false", workers: 2, maxPages: 10,
			pages: pagesUnder(seed, 3), wantVisits: 3, wantFetches: 3, wantLimitHit: false,
		},
		{
			// seed, a (fails, slot released), b, then c is dropped; 3 fetch attempts, 2 counted.
			name: "failed_fetch_does_not_count", workers: 1, maxPages: 2,
			pages:      map[string][]string{seed: {seed + "/a", seed + "/b", seed + "/c"}},
			failURLs:   map[string]bool{seed + "/a": true},
			wantVisits: 2, wantFetches: 3, wantLimitHit: true,
		},
		{
			name: "seed_fetch_fails_reports_false", workers: 1, maxPages: 1,
			pages:      pagesUnder(seed, 3),
			failURLs:   map[string]bool{seed: true},
			wantVisits: 0, wantFetches: 1, wantLimitHit: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mockStore{done: make(chan uuid.UUID, 4)}
			f := &mockFetcher{urlsToReturn: tc.pages, failURLs: tc.failURLs, delay: tc.delay}
			c := NewCrawler(zerolog.Nop(), tc.workers, tc.maxPages, f, s)
			runCrawler(t, c)

			id := uuid.New()
			seedURL := mustParse(t, seed)
			c.Submit(context.Background(), id, seedURL, seedURL)

			if got := waitCrawlDone(t, s); got != id {
				t.Fatalf("done crawl = %s, want %s", got, id)
			}

			rec := s.record(id)
			if rec.status != CrawlStatusDone {
				t.Errorf("status = %s, want %s", rec.status, CrawlStatusDone)
			}
			if rec.visits != tc.wantVisits {
				t.Errorf("visits = %d, want %d", rec.visits, tc.wantVisits)
			}
			if got := f.fetchCount(); got != tc.wantFetches {
				t.Errorf("fetches = %d, want %d", got, tc.wantFetches)
			}
			if rec.limitReached != tc.wantLimitHit {
				t.Errorf("limitReached = %v, want %v", rec.limitReached, tc.wantLimitHit)
			}
		})
	}
}

func TestCrawler_Run_MaxPages_OtherCrawlsContinue(t *testing.T) {
	const (
		seedA = "https://a.example.com/site"
		seedB = "https://b.example.com/site"
	)
	pages := pagesUnder(seedA, 10)
	for k, v := range pagesUnder(seedB, 3) {
		pages[k] = v
	}

	s := &mockStore{done: make(chan uuid.UUID, 4)}
	f := &mockFetcher{urlsToReturn: pages, delay: time.Millisecond}
	c := NewCrawler(zerolog.Nop(), 2, 3, f, s)
	runCrawler(t, c)

	idA, idB := uuid.New(), uuid.New()
	a, b := mustParse(t, seedA), mustParse(t, seedB)
	c.Submit(context.Background(), idA, a, a)
	c.Submit(context.Background(), idB, b, b)

	waitCrawlDone(t, s)
	waitCrawlDone(t, s)

	cases := []struct {
		name         string
		id           uuid.UUID
		wantVisits   int64
		wantLimitHit bool
	}{
		{"limited_crawl_stops_at_cap", idA, 3, true},
		{"other_crawl_finishes_all_pages", idB, 3, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := s.record(tc.id)
			if rec.status != CrawlStatusDone {
				t.Errorf("status = %s, want %s", rec.status, CrawlStatusDone)
			}
			if rec.visits != tc.wantVisits {
				t.Errorf("visits = %d, want %d", rec.visits, tc.wantVisits)
			}
			if rec.limitReached != tc.wantLimitHit {
				t.Errorf("limitReached = %v, want %v", rec.limitReached, tc.wantLimitHit)
			}
		})
	}
}

func TestCrawler_Run_MaxPages_LogsOnce(t *testing.T) {
	const seed = "https://example.com/site"

	var buf bytes.Buffer
	// Info level: no Debug lines from goroutines that outlive the done signal.
	logger := zerolog.New(zerolog.SyncWriter(&buf)).Level(zerolog.InfoLevel)

	s := &mockStore{done: make(chan uuid.UUID, 4)}
	f := &mockFetcher{urlsToReturn: pagesUnder(seed, 21), delay: time.Millisecond}
	c := NewCrawler(logger, 4, 5, f, s)
	stop := runCrawler(t, c)

	id := uuid.New()
	seedURL := mustParse(t, seed)
	c.Submit(context.Background(), id, seedURL, seedURL)
	waitCrawlDone(t, s)
	stop() // all goroutines that log have finished; safe to read buf

	var matches []map[string]any
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		if entry["message"] == "max pages per crawl reached" {
			matches = append(matches, entry)
		}
	}

	if len(matches) != 1 {
		t.Fatalf("limit log lines = %d, want 1", len(matches))
	}
	if matches[0]["ID"] != id.String() {
		t.Errorf("ID = %v, want %s", matches[0]["ID"], id)
	}
	if matches[0]["max_pages"] != float64(5) {
		t.Errorf("max_pages = %v, want 5", matches[0]["max_pages"])
	}
}
