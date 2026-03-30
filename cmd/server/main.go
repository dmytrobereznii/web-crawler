package main

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/dmytrobereznii/web-crawler/internal/fetcher"
	"github.com/dmytrobereznii/web-crawler/internal/store"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	"github.com/dmytrobereznii/web-crawler/internal/api"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	err := godotenv.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load .env")
	}

	crawlStore := store.NewCrawlStore()

	workersCountS := os.Getenv("WORKERS_COUNT")
	workersCount, err := strconv.Atoi(workersCountS)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get env variable WORKERS_COUNT")
	}
	if workersCount < 1 {
		logger.Fatal().Msg("WORKERS_COUNT must be greater than zero")
	}

	c := crawler.NewCrawler(logger, workersCount, fetcher.NewFetcher(), crawlStore)
	go c.Run(context.Background())

	h := api.NewHandler(logger, crawlStore, c)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawls", h.CreateCrawl)
	mux.HandleFunc("GET /crawls/{id}", h.GetCrawl)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Fatal().Err(err).Msg("failed to start server")
	}
}
