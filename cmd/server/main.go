package main

import (
	"net/http"
	"os"

	"github.com/rs/zerolog"

	"github.com/dmytrobereznii/web-crawler/internal/api"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	h := api.NewHandler(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawls", h.CreateCrawl)
	mux.HandleFunc("GET /crawls/{id}", h.GetCrawl)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Fatal().Err(err).Msg("failed to start server")
	}
}
