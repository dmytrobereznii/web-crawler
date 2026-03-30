package fetcher

import (
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

func (f *Fetcher) Fetch(u *url.URL) ([]*url.URL, time.Duration, error) {
	start := time.Now()
	resp, err := f.client.Get(u.String())
	dur := time.Since(start)
	if err != nil {
		return nil, dur, err
	}
	defer resp.Body.Close()

	var links []*url.URL

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

						links = append(links, u.ResolveReference(linkURL))
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
