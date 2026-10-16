package config

import (
	"strings"
	"testing"
)

func TestParseMaxPages(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "empty_is_unlimited", raw: "", want: 0},
		{name: "positive_is_limit", raw: "50", want: 50},
		{name: "zero_rejected", raw: "0", wantErr: true},
		{name: "negative_rejected", raw: "-5", wantErr: true},
		{name: "non_numeric_rejected", raw: "abc", wantErr: true},
		{name: "decimal_rejected", raw: "10.5", wantErr: true},
		{name: "surrounding_space_rejected", raw: " 10", wantErr: true},
		{name: "overflow_rejected", raw: "99999999999999999999", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMaxPages(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMaxPages(%q) = %d, nil; want error", tt.raw, got)
				}
				if !strings.Contains(err.Error(), "MAX_PAGES_PER_CRAWL") {
					t.Errorf("error %q does not name MAX_PAGES_PER_CRAWL", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMaxPages(%q) unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("ParseMaxPages(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
