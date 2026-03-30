package fetcher

import (
	"fmt"
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

func (f *Fetcher) Fetch(u *url.URL) (int, time.Duration, error) {
	start := time.Now()
	resp, err := f.client.Get(u.String())
	dur := time.Since(start)
	if err != nil {
		return 0, dur, err
	}
	defer resp.Body.Close()

	z := html.NewTokenizer(resp.Body)

loop:
	for {
		tt := z.Next()

		switch tt {
		case html.ErrorToken:
			if err = z.Err(); err == io.EOF {
				break loop
			}
			return resp.StatusCode, dur, err
		case html.StartTagToken:
			tn, _ := z.TagName()
			if len(tn) == 1 && tn[0] == 'a' {
				for {
					k, v, more := z.TagAttr()
					if string(k) == "href" {
						fmt.Println(string(v))
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

	return resp.StatusCode, dur, nil
}
