package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/dmytrobereznii/web-crawler/internal/store"
	"github.com/google/uuid"
)

type createCrawlRequest struct {
	URL string `json:"url"`
}

type createCrawlResponse struct {
	ID string `json:"id"`
}

func (h *Handler) CreateCrawl(w http.ResponseWriter, r *http.Request) {
	var request createCrawlRequest
	logger := h.logger(r.Context())
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeErrorResponse(w, logger, http.StatusBadRequest, err.Error())
		return
	}

	u, err := url.ParseRequestURI(request.URL)
	if err != nil {
		writeErrorResponse(w, logger, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		writeErrorResponse(w, logger, http.StatusUnprocessableEntity, "Scheme must be 'http' or 'https'")
		return
	}
	if u.Hostname() == "" {
		writeErrorResponse(w, logger, http.StatusUnprocessableEntity, "URL must have a hostname")
		return
	}

	crawl := crawler.Crawl{
		URL:    *u,
		ID:     uuid.New(),
		Status: crawler.CrawlStatusPending,
	}

	err = h.store.Save(crawl)
	if errors.Is(err, store.ErrAlreadyExists) {
		writeErrorResponse(w, logger, http.StatusUnprocessableEntity, err.Error())
		return
	} else if err != nil {
		writeErrorResponse(w, logger, http.StatusInternalServerError, err.Error())
		return
	}

	h.crawler.Submit(r.Context(), crawl.ID, crawl.URL, crawl.URL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createCrawlResponse{ID: crawl.ID.String()}); err != nil {
		logger.Error().Err(err).Msg("failed to encode response")
	}
}
