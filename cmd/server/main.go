package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/dmytrobereznii/web-crawler/internal/fetcher"
	"github.com/dmytrobereznii/web-crawler/internal/middleware"
	"github.com/dmytrobereznii/web-crawler/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/dmytrobereznii/web-crawler/internal/api"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	cfg.MaxConns = 20
	cfg.ConnConfig.RuntimeParams["application_name"] = "web-crawler"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	err = pool.Ping(ctx)
	if err != nil {
		return err
	}

	crawlStore := store.NewMemoryCrawlStore()
	//crawlStore := store.NewPostgresCrawlStore(pool)

	workersCountS := os.Getenv("WORKERS_COUNT")
	workersCount, err := strconv.Atoi(workersCountS)
	if err != nil {
		return err
	}
	if workersCount < 1 {
		return errors.New("WORKERS_COUNT must be greater than zero")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := crawler.NewCrawler(logger, workersCount, fetcher.NewFetcher(), crawlStore)

	h := api.NewHandler(logger, crawlStore, c)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawls", h.CreateCrawl)
	mux.HandleFunc("GET /crawls/{id}", h.GetCrawl)

	wrappedMux := middleware.Chain(mux, middleware.RequestIDMiddleware, middleware.LoggingMiddleware(logger))

	srv := &http.Server{Addr: ":8080", Handler: wrappedMux}
	go func() {
		if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("failed to start server")
		}
	}()

	c.Run(ctx)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = srv.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	return nil
}
