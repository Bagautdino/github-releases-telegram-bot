package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

// Open creates a new database connection and runs migrations
func Open(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}

	// Run migrations
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs database migrations
func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS repos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner TEXT NOT NULL,
			name TEXT NOT NULL,
			track_prereleases INTEGER NOT NULL DEFAULT 0,
			UNIQUE(owner, name)
		)`,
		`CREATE TABLE IF NOT EXISTS chats (
			id INTEGER PRIMARY KEY,
			title TEXT,
			language TEXT DEFAULT 'ru'
		)`,
		`CREATE TABLE IF NOT EXISTS processed_releases (
			repo_owner TEXT NOT NULL,
			repo_name  TEXT NOT NULL,
			release_id INTEGER NOT NULL,
			tag_name   TEXT,
			published_at TEXT,
			created_at   TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (repo_owner, repo_name, release_id)
		)`,
		`CREATE TABLE IF NOT EXISTS etags (
			repo_owner TEXT NOT NULL,
			repo_name  TEXT NOT NULL,
			etag       TEXT NOT NULL,
			updated_at TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (repo_owner, repo_name)
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, migration := range migrations {
		if _, err := db.conn.Exec(migration); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	return nil
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return db.conn.BeginTx(ctx, nil)
}
