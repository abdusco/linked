package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/abdusco/linked/internal"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

type ClickStats struct {
	Total         int64
	LastClickedAt *time.Time
}

type clickStatsRow struct {
	Total         int64 `db:"total"`
	LastClickedAt *Date `db:"last_clicked_at"`
}

func (r clickStatsRow) toDomain() *internal.LinkStats {
	var lastClickedAt *time.Time
	if r.LastClickedAt != nil {
		lastClickedAt = lo.ToPtr(r.LastClickedAt.Time())
	}
	return &internal.LinkStats{
		Clicks:        r.Total,
		LastClickedAt: lastClickedAt,
	}
}

type ClicksRepo struct {
	db *goqu.Database
}

func NewClicksRepo(db *sql.DB) *ClicksRepo {
	return &ClicksRepo{db: goqu.New("sqlite", db)}
}

func (r *ClicksRepo) Create(ctx context.Context, linkID int64, userAgent, ipAddress string) error {
	now := Date(time.Now().UTC())
	query := r.db.Insert("clicks").
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

func (r *ClicksRepo) GetStatsForLink(ctx context.Context, linkID int64) (*internal.LinkStats, any) {
	query := r.db.From("clicks").
		Where(goqu.I("link_id").Eq(linkID)).
		Select(
			goqu.COUNT("*").As("total"),
			goqu.MAX("clicked_at").As("last_clicked_at"),
		)

	var row clickStatsRow
	found, err := query.ScanStructContext(ctx, &row)
	if err != nil {
		return nil, fmt.Errorf("failed to scan links stats: %w", err)
	} else if !found {
		return nil, internal.ErrLinkNotFound
	}

	return row.toDomain(), nil
}
