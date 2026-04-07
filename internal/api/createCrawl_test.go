package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmytrobereznii/web-crawler/internal/api"
	"github.com/dmytrobereznii/web-crawler/internal/store"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type createCrawlResponse struct {
	ID string `json:"id"`
}

func TestHandler_CreateCrawl(t *testing.T) {
	cases := []struct {
		name    string
		reqJson string
		strErr  error
		code    int
	}{
		{"invalid_json_returns_400", `{"url":"corrupted`, nil, http.StatusBadRequest},
		{"invalid_url_returns_422", `{"url":"abc"}`, nil, http.StatusUnprocessableEntity},
		{"wrong_scheme_returns_422", `{"url":"ftp://google.com"}`, nil, http.StatusUnprocessableEntity},
		{"missing_hostname_returns_422", `{"url":"https://"}`, nil, http.StatusUnprocessableEntity},
		{"store_record_already_exists_returns_422", `{"url":"https://github.com/joho/godotenv/tree/main"}`, store.ErrAlreadyExists, http.StatusUnprocessableEntity},
		{"store_failure_returns_500", `{"url":"https://github.com/joho/godotenv/tree/main"}`, errors.New("unexpected store failure"), http.StatusInternalServerError},
		{"success_returns_201", `{"url":"https://github.com/joho/godotenv/tree/main"}`, nil, http.StatusCreated},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mockStore{err: tc.strErr}
			c := &mockCrawler{}
			handler := api.NewHandler(
				zerolog.Nop(),
				s,
				c,
			)

			body := strings.NewReader(tc.reqJson)
			req := httptest.NewRequest(http.MethodPost, "/crawls", body)

			rec := httptest.NewRecorder()
			handler.CreateCrawl(rec, req)

			if rec.Code != tc.code {
				t.Errorf("unexpected code: got %d, want %d", rec.Code, tc.code)
			}

			if tc.code == http.StatusCreated {
				var got createCrawlResponse
				err := json.NewDecoder(rec.Body).Decode(&got)

				if err != nil {
					t.Errorf("unexpected error when decoding response: %v", err)
				}
				err = uuid.Validate(got.ID)
				if err != nil {
					t.Errorf("invalid ID returned: %v", err)
				}
				if !c.called {
					t.Error("crawlSubmitter.Submit must called")
				}
			}
		})
	}
}
