package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/dmytrobereznii/web-crawler/internal/store"
	"github.com/google/uuid"
)

type getCrawlResponse struct {
	ID       string              `json:"id"`
	Status   crawler.CrawlStatus `json:"status"`
	Duration int64               `json:"duration"`
	Visits   int64               `json:"visits"`
}

func (h *Handler) GetCrawl(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	logger := h.logger(r.Context())

	if err != nil {
		writeErrorResponse(w, logger, http.StatusUnprocessableEntity, err.Error())
		return
	}

	crawl, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrorResponse(w, logger, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		writeErrorResponse(w, logger, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := getCrawlResponse{
		ID:       crawl.ID.String(),
		Status:   crawl.Status,
		Duration: crawl.Duration,
		Visits:   crawl.Visits,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error().Err(err).Msg("failed to encode response")
	}
}
