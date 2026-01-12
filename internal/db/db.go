package db

import (
	"context"
	"database/sql"
	"net/url"
	"sync"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

var (
	instance *sql.DB
	once     sync.Once
)

func Init(ctx context.Context, dbPath string) (*sql.DB, error) {
	dsn := formatDBPath(dbPath)
	var err error
	once.Do(func() {
		instance, err = sql.Open("sqlite", dsn)
		if err != nil {
			log.Error().Err(err).Msg("failed to open database")
			return
		}

		err = instance.PingContext(ctx)
		if err != nil {
			log.Error().Err(err).Msg("failed to ping database")
			return
		}

		log.Debug().Msg("database connection successful")

		err = migrate(ctx, instance)
		if err != nil {
			log.Error().Err(err).Msg("failed to run migrations")
		} else {
			log.Info().Msg("migrations completed successfully")
		}
	})
	return instance, err
}

func formatDBPath(path string) string {
	// Add pragmas for better performance and safety
	// See: https://pkg.go.dev/modernc.org/sqlite#pkg-overview
	params := url.Values{}
	params.Set("cache", "shared")
	params.Set("mode", "rwc")
	params.Set("_time_format", "sqlite")
	params.Set("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Set("_busy_timeout", "5000")

	return "file:" + path + "?" + params.Encode()
}

func migrate(ctx context.Context, db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT UNIQUE NOT NULL,
		url TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS clicks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		link_id INTEGER NOT NULL,
		clicked_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		user_agent TEXT,
		ip_address TEXT,
		FOREIGN KEY(link_id) REFERENCES links(id) ON DELETE CASCADE
	);	

	CREATE INDEX IF NOT EXISTS idx_links_slug ON links(slug);
	CREATE INDEX IF NOT EXISTS idx_clicks_link_id ON clicks(link_id);
	CREATE INDEX IF NOT EXISTS idx_clicks_clicked_at ON clicks(clicked_at);
	`

	_, err := db.ExecContext(ctx, schema)
	return err
}
