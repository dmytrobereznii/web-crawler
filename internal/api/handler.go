package api

import (
	"context"
	"net/url"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/dmytrobereznii/web-crawler/internal/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Handler struct {
	fallbackLogger zerolog.Logger
	store          crawlStore
	crawler        crawlSubmitter
}

func NewHandler(logger zerolog.Logger, store crawlStore, crawler crawlSubmitter) *Handler {
	return &Handler{
		fallbackLogger: logger,
		store:          store,
		crawler:        crawler,
	}
}

func (h *Handler) logger(ctx context.Context) zerolog.Logger {
	middlewareLogger, ok := middleware.Logger(ctx)

	if !ok {
		h.fallbackLogger.Error().Msg("Failed to get fallbackLogger from context")

		return h.fallbackLogger
	}

	return middlewareLogger
}

type crawlStore interface {
	Get(context.Context, uuid.UUID) (crawler.Crawl, error)
	Save(context.Context, crawler.Crawl) error
}

type crawlSubmitter interface {
	Submit(context.Context, uuid.UUID, url.URL, url.URL)
}
