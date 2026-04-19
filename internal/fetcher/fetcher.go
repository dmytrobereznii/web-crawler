package fetcher

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/html"
)

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (f *Fetcher) Fetch(ctx context.Context, u url.URL) ([]url.URL, time.Duration, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := f.client.Do(req)
	dur := time.Since(start)
	if err != nil {
		return nil, dur, err
	}
	defer resp.Body.Close()

	var links []url.URL

	z := html.NewTokenizer(resp.Body)

loop:
	for {
		tt := z.Next()

		switch tt {
		case html.ErrorToken:
			if err = z.Err(); err == io.EOF {
				break loop
			}
			return nil, dur, err
		case html.StartTagToken:
			tn, _ := z.TagName()
			if len(tn) == 1 && tn[0] == 'a' {
				for {
					k, v, more := z.TagAttr()
					if string(k) == "href" {
						linkURL, err := url.Parse(string(v))
						if err != nil {
							continue
						}

						targetURL := u.ResolveReference(linkURL)
						targetURL.Fragment = ""
						targetURL.RawFragment = ""
						links = append(links, *targetURL)
						break
					}
					if !more {
						break
					}
				}
			}
		default:
		}
	}

	return links, dur, nil
}
