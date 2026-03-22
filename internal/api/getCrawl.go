package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

type getCrawlResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (h *Handler) GetCrawl(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErrorResponse(w, h.logger, http.StatusUnprocessableEntity, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := getCrawlResponse{
		ID:     id.String(),
		Status: "ok",
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error().Err(err).Msg("failed to encode response")
	}
}
