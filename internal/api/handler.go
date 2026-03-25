package api

import (
	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Handler struct {
	logger zerolog.Logger
	store  crawlStore
}

func NewHandler(logger zerolog.Logger, store crawlStore) *Handler {
	return &Handler{
		logger: logger,
		store:  store,
	}
}

type crawlStore interface {
	Get(uuid.UUID) (crawler.Crawl, error)
	Save(crawl crawler.Crawl) error
}
