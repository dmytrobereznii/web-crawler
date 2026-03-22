package api

import (
	"github.com/rs/zerolog"
)

type Handler struct {
	logger zerolog.Logger

	// store will go here later
}

func NewHandler(logger zerolog.Logger) *Handler {
	return &Handler{logger}
}
