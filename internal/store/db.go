package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yash121l/Vessel/internal/logger"
	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection with Vessel-specific methods.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at the given path.
func Open(path string) (*DB, error) {
	logger.Infof("Opening SQLite database at path: %s", path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		logger.Errorf("failed to create database directory for path %s: %v", path, err)
		return nil, fmt.Errorf("cannot create db directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		logger.Errorf("failed to open SQLite connection at %s: %v", path, err)
		return nil, fmt.Errorf("cannot open database: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	logger.Infof("Successfully opened SQLite database connection")
	return &DB{db}, nil
}

// Migrate runs all schema migrations.
func (db *DB) Migrate() error {
	logger.Infof("Running database schema migrations...")
	_, err := db.Exec(schema)
	if err != nil {
		logger.Errorf("initial schema migration failed: %v", err)
		return err
	}
	// Run additive migrations for existing databases
	migrations := []string{
		`ALTER TABLE deployments ADD COLUMN imported INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE deployments ADD COLUMN container_id TEXT`,
		`ALTER TABLE deployments ADD COLUMN image TEXT`,
		`ALTER TABLE deployments ADD COLUMN ports TEXT`,
		`ALTER TABLE users ADD COLUMN last_login_at DATETIME`,
		`ALTER TABLE operations ADD COLUMN resource_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE operations ADD COLUMN metadata TEXT NOT NULL DEFAULT ''`,
	}
	for _, m := range migrations {
		// Ignore errors — column likely already exists
		logger.Debugf("Applying additive migration if needed: %s", m)
		_, _ = db.Exec(m)
	}
	logger.Infof("Database migrations completed successfully")
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS deployments (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    app_id       TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'stopped',
    domain       TEXT,
    compose_dir  TEXT NOT NULL,
    imported     INTEGER NOT NULL DEFAULT 0,
    container_id TEXT,
    image        TEXT,
    ports        TEXT,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
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

CREATE TABLE IF NOT EXISTS users (
    id               TEXT PRIMARY KEY,
    username         TEXT NOT NULL UNIQUE,
    role             TEXT NOT NULL,
    password_hash    TEXT NOT NULL,
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login_at    DATETIME
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS operations (
    id            TEXT PRIMARY KEY,
    kind          TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   TEXT,
    resource_name TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'queued',
    summary       TEXT NOT NULL DEFAULT '',
    error         TEXT NOT NULL DEFAULT '',
    actor_user_id TEXT NOT NULL DEFAULT '',
    actor_username TEXT NOT NULL DEFAULT '',
    metadata      TEXT NOT NULL DEFAULT '',
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at    DATETIME,
    finished_at   DATETIME,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS operation_steps (
    id          TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL DEFAULT 0,
    step_key    TEXT NOT NULL,
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    details     TEXT NOT NULL DEFAULT '',
    output      TEXT NOT NULL DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at  DATETIME,
    finished_at DATETIME,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS deployments_updated_at
    AFTER UPDATE ON deployments
    BEGIN
        UPDATE deployments SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS users_updated_at
    AFTER UPDATE ON users
    BEGIN
        UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS operations_updated_at
    AFTER UPDATE ON operations
    BEGIN
        UPDATE operations SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS operation_steps_updated_at
    AFTER UPDATE ON operation_steps
    BEGIN
        UPDATE operation_steps SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;
`
