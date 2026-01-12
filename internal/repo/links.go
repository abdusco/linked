package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/abdusco/linked/internal"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type linkRow struct {
	ID        int64  `db:"id" goqu:"skipinsert,skipupdate"`
	Slug      string `db:"slug"`
	URL       string `db:"url"`
	CreatedAt Date   `db:"created_at" goqu:"skipupdate"`
}

type LinksRepo struct {
	db *goqu.Database
}

func NewLinksRepo(db *sql.DB) *LinksRepo {
	return &LinksRepo{db: goqu.New("sqlite", db)}
}

func (r *LinksRepo) Create(ctx context.Context, slug, url string) (*internal.Link, error) {
	q := r.db.Insert("links").
		Rows(linkRow{
			Slug:      slug,
			URL:       url,
			CreatedAt: Date(time.Now().UTC()),
		}).
		Returning(linkRow{})

	var row linkRow
	found, err := q.Executor().ScanStructContext(ctx, &row)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, internal.ErrSlugExists
		}
		return nil, fmt.Errorf("failed to insert link: %w", err)
	} else if !found {
		return nil, errors.New("insert did not return anything")
	}

	link := row.toDomain()

	return link, nil
}

func (r *LinksRepo) GetBySlug(ctx context.Context, slug string) (*internal.Link, error) {
	q := r.db.
		From("links").
		Where(goqu.I("slug").Eq(slug)).
		Select(linkRow{})

	var row linkRow
	found, err := q.ScanStructContext(ctx, &row)
	if err != nil {
		return nil, fmt.Errorf("failed to scan link: %w", err)
	} else if !found {
		return nil, internal.ErrLinkNotFound
	}

	return row.toDomain(), nil
}

func (r *LinksRepo) ListAll(ctx context.Context) ([]*internal.Link, error) {
	query := r.db.From("links").
		Select(linkRow{}).
		Order(goqu.C("id").Desc())

	var rows []linkRow
	err := query.Executor().ScanStructsContext(ctx, &rows)
	if err != nil {
		return nil, err
	}

	clicksRepo := NewClicksRepo(r.db.Db.(*sql.DB))

	links := make([]*internal.Link, len(rows))
	for i, row := range rows {
		link := row.toDomain()

		stats, err := clicksRepo.GetStatsForLink(ctx, link.ID)
		if err == nil {
			link.Stats = stats
		}

		links[i] = link
	}

	return links, nil
}

func (r *LinksRepo) Delete(ctx context.Context, id int64) error {
	query := r.db.From("links").
		Where(goqu.I("id").Eq(id)).
		Delete()

	result, err := query.Executor().ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	} else if n == 0 {
		return internal.ErrLinkNotFound
	}

	return nil
}

func (r *linkRow) toDomain() *internal.Link {
	return &internal.Link{
		ID:        r.ID,
		Slug:      r.Slug,
		URL:       r.URL,
		CreatedAt: r.CreatedAt.Time(),
	}
}

func GenerateSlug() string {
	const charset = "abcdefghjkmnopqrstuvwxyz0123456789"
	const slugLength = 6
	slug := make([]byte, slugLength)
	for i := range slug {
		slug[i] = charset[rand.Intn(len(charset))]
	}
	return string(slug)
}

func isUniqueConstraintError(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
}
