package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/dmytrobereznii/web-crawler/internal/fetcher"
	"github.com/dmytrobereznii/web-crawler/internal/middleware"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := crawler.NewCrawler(logger, workersCount, fetcher.NewFetcher(), crawlStore)

	h := api.NewHandler(logger, crawlStore, c)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawls", h.CreateCrawl)
	mux.HandleFunc("GET /crawls/{id}", h.GetCrawl)
	wrappedMux := middleware.RequestIDMiddleware(middleware.LoggingMiddleware(logger)(mux))

	srv := &http.Server{Addr: ":8080", Handler: wrappedMux}
	go func() {
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	c.Run(ctx)

	err = srv.Shutdown(context.Background())
	if err != nil {
		return
	}
}
