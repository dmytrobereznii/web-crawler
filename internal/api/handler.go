package api

import (
	"context"
	"net/url"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Handler struct {
	logger  zerolog.Logger
	store   crawlStore
	crawler crawlSubmitter
}

func NewHandler(logger zerolog.Logger, store crawlStore, crawler crawlSubmitter) *Handler {
	return &Handler{
		logger:  logger,
		store:   store,
		crawler: crawler,
	}
}

type crawlStore interface {
	Get(uuid.UUID) (crawler.Crawl, error)
	Save(crawl crawler.Crawl) error
}

type crawlSubmitter interface {
	Submit(ctx context.Context, u *url.URL)
}
