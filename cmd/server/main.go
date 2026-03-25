package main

import (
	"net/http"
	"os"

	"github.com/dmytrobereznii/web-crawler/internal/store"
	"github.com/rs/zerolog"

	"github.com/dmytrobereznii/web-crawler/internal/api"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	crawlStore := store.NewCrawlStore()

	h := api.NewHandler(logger, crawlStore)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawls", h.CreateCrawl)
	mux.HandleFunc("GET /crawls/{id}", h.GetCrawl)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Fatal().Err(err).Msg("failed to start server")
	}
}
