package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeErrorResponse(w http.ResponseWriter, logger zerolog.Logger, status int, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := errorResponse{
		Error: err,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error().Err(err).Msg("failed to encode response")
	}
}
