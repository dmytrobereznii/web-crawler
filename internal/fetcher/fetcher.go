package fetcher

import (
	"net/http"
	"net/url"
	"time"
)

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (f *Fetcher) Fetch(u *url.URL) (int, time.Duration, error) {
	start := time.Now()
	resp, err := f.client.Get(u.String())
	dur := time.Since(start)
	if err != nil {
		return 0, dur, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, dur, nil
}
