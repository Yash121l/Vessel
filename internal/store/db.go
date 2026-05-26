package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection with Vessel-specific methods.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at the given path.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("cannot create db directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	return &DB{db}, nil
}

// Migrate runs all schema migrations.
func (db *DB) Migrate() error {
	_, err := db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS deployments (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    app_id      TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'stopped',
    domain      TEXT,
    compose_dir TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS deployment_env (
    deployment_id TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    key           TEXT NOT NULL,
    value         TEXT NOT NULL,
    PRIMARY KEY (deployment_id, key)
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TRIGGER IF NOT EXISTS deployments_updated_at
    AFTER UPDATE ON deployments
    BEGIN
        UPDATE deployments SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;
`
