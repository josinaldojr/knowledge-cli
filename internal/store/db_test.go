package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenConfiguresDatabaseAndMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='hook_events'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "hook_events" {
		t.Fatalf("table = %q", name)
	}
}

func TestMigrationBacksUpExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB.Exec(`CREATE TABLE existing_data (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 1 {
		t.Fatalf("backup count = %d", len(entries))
	}
}

func TestWithinTxRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB.Exec(`CREATE TABLE records (value TEXT)`); err != nil {
		t.Fatal(err)
	}
	err = db.WithinTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO records(value) VALUES ('discarded')`); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("WithinTx() error = nil")
	}
	var count int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM records`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rollback count = %d", count)
	}
}
