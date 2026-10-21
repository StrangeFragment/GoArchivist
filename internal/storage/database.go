package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens the pure-Go SQLite database and ensures the folder catalog schema
// exists. Keeping this driver CGO-free preserves Windows cross-compilation.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("путь к базе данных не задан")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("не удалось создать папку базы данных: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("не удалось подключиться к SQLite: %w", err)
	}

	if _, err := db.Exec(`
		PRAGMA foreign_keys = ON;
		PRAGMA busy_timeout = 5000;
		CREATE TABLE IF NOT EXISTS mailboxes (
			id INTEGER PRIMARY KEY,
			imap_name TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			is_watched INTEGER NOT NULL DEFAULT 0 CHECK (is_watched IN (0, 1)),
			is_available INTEGER NOT NULL DEFAULT 1 CHECK (is_available IN (0, 1)),
			discovered_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			uid_validity INTEGER,
			last_uid INTEGER,
			last_synced_at TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_mailboxes_watched_available
			ON mailboxes(is_watched, is_available);
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("не удалось создать схему SQLite: %w", err)
	}

	return db, nil
}
