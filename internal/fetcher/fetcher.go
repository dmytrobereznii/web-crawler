package fetcher

import (
	"net/http"
	"net/url"
)

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{},
	}
}

func (f *Fetcher) Fetch(u *url.URL) (int, error) {
	resp, err := f.client.Get(u.String())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}
