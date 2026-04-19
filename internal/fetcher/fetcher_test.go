package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
)

func TestFetcher_Fetch(t *testing.T) {
	cases := []struct {
		name     string
		html     string
		expected func(_ string) []string
	}{
		{"valid_urls", `<html><body><a href="https://example.com">link</a></body></html>`, func(_ string) []string {
			return []string{"https://example.com"}
		}},
		{"relative_urls", `<html><body><a href="/contact">link</a></body></html>`, func(srvURL string) []string {
			return []string{srvURL + "/contact"}
		}},
		{"malformed_html", `<html><body..."https://example.com">link</a></body></html>`, nil},
		{"no_link", `<html><body></body></html>`, nil},
		{"multiple_urls", `<html><body><a href="https://example.com/page-1">1</a><a href="https://example.com/page-2">2</a></body></html>`, func(srvURL string) []string {
			return []string{"https://example.com/page-1", "https://example.com/page-2"}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher := NewFetcher()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, tc.html)
			}))
			defer srv.Close()

			srvURL, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatal(err)
			}

			foundURLs, _, _ := fetcher.Fetch(context.Background(), *srvURL)

			if tc.expected == nil && len(foundURLs) != 0 {
				t.Errorf("unexpected found URLs: %v", foundURLs)
			}

			if tc.expected != nil {
				gotURLs := make([]string, len(foundURLs))
				for i, u := range foundURLs {
					gotURLs[i] = u.String()
				}

				wantURLs := tc.expected(srv.URL)

				if !slices.Equal(wantURLs, gotURLs) {
					t.Errorf("unexpected URLs: want %v, got %v", wantURLs, gotURLs)
				}
			}
		})
	}
}
