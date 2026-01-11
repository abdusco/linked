package db

import (
	"database/sql"
	"sync"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

var (
	instance *sql.DB
	once     sync.Once
)

func Init(dbPath string) (*sql.DB, error) {
	var err error
	once.Do(func() {
		instance, err = sql.Open("sqlite", dbPath)
		if err != nil {
			log.Error().Err(err).Msg("failed to open database")
			return
		}

		err = instance.Ping()
		if err != nil {
			log.Error().Err(err).Msg("failed to ping database")
			return
		}

		log.Debug().Msg("database connection successful")

		err = migrate(instance)
		if err != nil {
			log.Error().Err(err).Msg("failed to run migrations")
		} else {
			log.Info().Msg("migrations completed successfully")
		}
	})
	return instance, err
}

func Get() *sql.DB {
	return instance
}

func migrate(db *sql.DB) error {
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

	_, err := db.Exec(schema)
	return err
}
