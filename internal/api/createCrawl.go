package api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

type crawlRequest struct {
	URL string `json:"url"`
}

type crawlResponse struct {
	ID string `json:"id"`
}

func (h *Handler) CreateCrawl(w http.ResponseWriter, r *http.Request) {
	var request crawlRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, err := url.ParseRequestURI(request.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		http.Error(w, "Scheme must be 'http' or 'https'", http.StatusUnprocessableEntity)
		return
	}
	if u.Hostname() == "" {
		http.Error(w, "URL must have a hostname", http.StatusUnprocessableEntity)
		return
	}

	id := uuid.New()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(crawlResponse{ID: id.String()}); err != nil {
		log.Printf("encode response: %v", err)
	}
}
