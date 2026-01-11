package repo

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	"github.com/rs/zerolog/log"
)

type Link struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	URL       string `json:"url"`
	CreatedAt Date   `json:"created_at"`
	Clicks    int64  `json:"clicks"`
	LastClick *Date  `json:"last_clicked_at"`
}

type linkRow struct {
	ID        int64  `db:"id" goqu:"skipinsert,skipupdate"`
	Slug      string `db:"slug"`
	URL       string `db:"url"`
	CreatedAt Date   `db:"created_at" goqu:"skipupdate"`
}

type LinksRepo struct {
	db *sql.DB
}

func NewLinksRepo(db *sql.DB) *LinksRepo {
	return &LinksRepo{db: db}
}

func (r *LinksRepo) Create(ctx context.Context, slug, url string) (*Link, error) {
	executor := goqu.New("sqlite", r.db)

	log.Debug().Str("slug", slug).Str("url", url).Msg("creating link")

	now := Date(time.Now().UTC())
	query := executor.Insert("links").
		Cols("slug", "url", "created_at").
		Vals([]any{slug, url, now}).
		Returning("id", "slug", "url", "created_at")

	var row linkRow
	found, err := query.Executor().ScanStructContext(ctx, &row)
	if err != nil {
		log.Error().Err(err).Str("slug", slug).Msg("failed to create link")
		return nil, err
	}

	if !found {
		log.Warn().Str("slug", slug).Msg("link creation returned no rows")
		return nil, errors.New("failed to create link")
	}

	link := row.toDomain()
	log.Info().Int64("id", link.ID).Str("slug", link.Slug).Msg("link created successfully")

	return link, nil
}

func (r *LinksRepo) GetBySlug(ctx context.Context, slug string) (*Link, error) {
	executor := goqu.New("sqlite", r.db)

	log.Debug().Str("slug", slug).Msg("fetching link by slug")

	query := executor.From("links").Where(goqu.Ex{"slug": slug}).Select(
		"id", "slug", "url", "created_at",
	)

	var row linkRow
	found, err := query.Executor().ScanStructContext(ctx, &row)
	if err != nil {
		log.Error().Err(err).Str("slug", slug).Msg("failed to fetch link")
		return nil, err
	}

	if !found {
		log.Debug().Str("slug", slug).Msg("link not found")
		return nil, errors.New("link not found")
	}

	link := row.toDomain()

	stats, _ := NewClicksRepo(r.db).GetStatsForLink(ctx, link.ID)
	if stats != nil {
		link.Clicks = stats.Total
		link.LastClick = stats.LastClickedAt
	}

	log.Debug().Int64("id", link.ID).Str("slug", slug).Int64("clicks", link.Clicks).Msg("link fetched")

	return link, nil
}

func (r *LinksRepo) ListAll(ctx context.Context) ([]*Link, error) {
	executor := goqu.New("sqlite", r.db)

	query := executor.From("links").Select(
		"id", "slug", "url", "created_at",
	).Order(goqu.C("created_at").Desc())

	var rows []linkRow
	err := query.Executor().ScanStructsContext(ctx, &rows)
	if err != nil {
		return nil, err
	}

	links := make([]*Link, len(rows))
	for i, row := range rows {
		link := row.toDomain()

		// Get click stats
		stats, _ := NewClicksRepo(r.db).GetStatsForLink(ctx, link.ID)
		if stats != nil {
			link.Clicks = stats.Total
			link.LastClick = stats.LastClickedAt
		}

		links[i] = link
	}

	return links, nil
}

func (r *linkRow) toDomain() *Link {
	return &Link{
		ID:        r.ID,
		Slug:      r.Slug,
		URL:       r.URL,
		CreatedAt: r.CreatedAt,
		Clicks:    0,
		LastClick: nil,
	}
}

func GenerateSlug() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	slug := make([]byte, 6)
	for i := range slug {
		slug[i] = charset[rand.Intn(len(charset))]
	}
	return string(slug)
}
