package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/dmytrobereznii/web-crawler/internal/crawler"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCrawlStore struct {
	pool *pgxpool.Pool
}

func NewPostgresCrawlStore(pool *pgxpool.Pool) *PostgresCrawlStore {
	return &PostgresCrawlStore{pool: pool}
}

func (cs *PostgresCrawlStore) Get(ctx context.Context, id uuid.UUID) (crawler.Crawl, error) {
	var status, urlStr string
	var duration pgtype.Interval
	var visits int64

	err := cs.pool.QueryRow(ctx,
		"SELECT status, url, duration, visits FROM crawl_jobs WHERE id = $1",
		id,
	).Scan(&status, &urlStr, &duration, &visits)
	if errors.Is(err, pgx.ErrNoRows) {
		return crawler.Crawl{}, ErrNotFound
	}
	if err != nil {
		return crawler.Crawl{}, fmt.Errorf("get crawl: %w", err)
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return crawler.Crawl{}, fmt.Errorf("parse url %q: %w", urlStr, err)
	}

	return crawler.Crawl{
		ID:       id,
		Status:   crawler.CrawlStatus(status),
		URL:      *parsedURL,
		Duration: time.Duration(duration.Microseconds) * time.Microsecond,
		Visits:   visits,
	}, nil
}

func (cs *PostgresCrawlStore) Save(ctx context.Context, crawl crawler.Crawl) error {
	duration := pgtype.Interval{
		Microseconds: crawl.Duration.Microseconds(),
		Valid:        true,
	}

	_, err := cs.pool.Exec(ctx,
		`INSERT INTO crawl_jobs (id, status, url, duration, visits, created_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, now(), null)`,
		crawl.ID, string(crawl.Status), crawl.URL.String(), duration, crawl.Visits,
	)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("save crawl: %w", err)
	}

	return nil
}

func (cs *PostgresCrawlStore) UpdateStatus(ctx context.Context, id uuid.UUID, status crawler.CrawlStatus) error {
	tag, err := cs.pool.Exec(ctx,
		"UPDATE crawl_jobs SET status = $1 WHERE id = $2",
		string(status), id,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (cs *PostgresCrawlStore) UpdateResult(ctx context.Context, id uuid.UUID, duration time.Duration, visits int64) error {
	interval := pgtype.Interval{
		Microseconds: duration.Microseconds(),
		Valid:        true,
	}

	tag, err := cs.pool.Exec(ctx,
		"UPDATE crawl_jobs SET duration = $1, visits = $2 WHERE id = $3",
		interval, visits, id,
	)
	if err != nil {
		return fmt.Errorf("update result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
