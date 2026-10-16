// Package config parses and validates environment-driven settings.
// Parsing lives here, not in cmd/server, so it is a pure function a
// table-driven test can call without a database.
package config

import (
	"errors"
	"fmt"
	"strconv"
)

// errNotPositive is unexported: callers only need the message, not to inspect it.
var errNotPositive = errors.New("must be a positive integer")

// ParseMaxPages converts the raw MAX_PAGES_PER_CRAWL value into a page limit.
// An empty string means "no limit" and returns 0; any other value must be a
// positive base-10 integer that fits in an int.
func ParseMaxPages(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	// Atoi rejects whitespace, decimals, text and out-of-range values for us.
	n, err := strconv.Atoi(raw)
	if err != nil {
		// %w keeps the *strconv.NumError in the chain for errors.Is/As.
		return 0, fmt.Errorf("MAX_PAGES_PER_CRAWL: %w", err)
	}
	if n < 1 {
		return 0, fmt.Errorf("MAX_PAGES_PER_CRAWL %q: %w", raw, errNotPositive)
	}
	return n, nil
}
