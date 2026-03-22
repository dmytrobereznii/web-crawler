package api

import (
	"encoding/json"
	"net/http"
	"net/url"

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
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeErrorResponse(w, h.logger, http.StatusBadRequest, err.Error())
		return
	}

	u, err := url.ParseRequestURI(request.URL)
	if err != nil {
		writeErrorResponse(w, h.logger, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		writeErrorResponse(w, h.logger, http.StatusUnprocessableEntity, "Scheme must be 'http' or 'https'")
		return
	}
	if u.Hostname() == "" {
		writeErrorResponse(w, h.logger, http.StatusUnprocessableEntity, "URL must have a hostname")
		return
	}

	id := uuid.New()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createCrawlResponse{ID: id.String()}); err != nil {
		h.logger.Error().Err(err).Msg("failed to encode response")
	}
}
