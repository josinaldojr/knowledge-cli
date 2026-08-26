package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct{ DB *sql.DB }

func Open(ctx context.Context, path string) (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("secure database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// A single connection avoids in-process writer contention. WAL and the busy
	// timeout still coordinate independent MCP stdio processes.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	// Set the timeout before WAL can acquire a database lock from another MCP process.
	for _, statement := range []string{"PRAGMA busy_timeout = 5000", "PRAGMA foreign_keys = ON", "PRAGMA journal_mode = WAL"} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure database: %w", err)
		}
	}
	if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
		db.Close()
		return nil, fmt.Errorf("secure database: %w", err)
	}
	return &Database{DB: db}, nil
}

func (d *Database) Close() error {
	if d == nil || d.DB == nil {
		return nil
	}
	return d.DB.Close()
}

func (d *Database) WithinTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
