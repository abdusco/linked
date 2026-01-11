package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	"github.com/rs/zerolog/log"
)

type ClickStats struct {
	Total         int64
	LastClickedAt *Date
}

type clickRow struct {
	ID        int64  `db:"id"`
	LinkID    int64  `db:"link_id"`
	ClickedAt Date   `db:"clicked_at"`
	UserAgent string `db:"user_agent"`
	IPAddress string `db:"ip_address"`
}

type clickStatsRow struct {
	Total         int64 `db:"total"`
	LastClickedAt *Date `db:"last_clicked_at"`
}

type ClicksRepo struct {
	db *sql.DB
}

func NewClicksRepo(db *sql.DB) *ClicksRepo {
	return &ClicksRepo{db: db}
}

func (r *ClicksRepo) Create(ctx context.Context, linkID int64, userAgent, ipAddress string) error {
	executor := goqu.New("sqlite", r.db)

	log.Debug().Int64("link_id", linkID).Str("ip", ipAddress).Msg("recording click")

	now := Date(time.Now().UTC())
	query := executor.Insert("clicks").
		Cols("link_id", "clicked_at", "user_agent", "ip_address").
		Vals([]any{linkID, now, userAgent, ipAddress}).
		Returning("id", "link_id", "clicked_at", "user_agent", "ip_address")

	_, err := query.Executor().ExecContext(ctx)
	if err != nil {
		log.Error().Err(err).Int64("link_id", linkID).Msg("failed to record click")
		return err
	}

	log.Debug().Int64("link_id", linkID).Str("ip", ipAddress).Msg("click recorded successfully")
	return nil
}

func (r *ClicksRepo) GetStatsForLink(ctx context.Context, linkID int64) (*ClickStats, any) {
	executor := goqu.New("sqlite", r.db)

	query := executor.From("clicks").Where(goqu.Ex{"link_id": linkID}).Select(
		goqu.COUNT("*").As("total"),
		goqu.MAX("clicked_at").As("last_clicked_at"),
	)

	var row clickStatsRow
	found, err := query.ScanStructContext(ctx, &row)
	if err != nil {
		return nil, err
	}

	if !found {
		return (&clickStatsRow{}).toDomain(), nil
	}

	return row.toDomain(), nil
}

func (r *clickStatsRow) toDomain() *ClickStats {
	return &ClickStats{
		Total:         r.Total,
		LastClickedAt: r.LastClickedAt,
	}
}
