package store

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	multiprocessWorkerEnv = "KV_MULTIPROCESS_WORKER"
	multiprocessDBEnv     = "KV_MULTIPROCESS_DB"
	multiprocessRows      = 8
	multiprocessWorkers   = 4
)

func TestOpenAcrossProcesses(t *testing.T) {
	if worker := os.Getenv(multiprocessWorkerEnv); worker != "" {
		writeFromSeparateProcess(t, os.Getenv(multiprocessDBEnv), worker)
		return
	}

	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	var journalMode string
	if err := db.DB.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if journalMode != "wal" {
		_ = db.Close()
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}
	var busyTimeout int
	if err := db.DB.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if busyTimeout != 5000 {
		_ = db.Close()
		t.Fatalf("busy timeout = %d, want 5000", busyTimeout)
	}
	if err := db.Migrate(context.Background(), path); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(`CREATE TABLE multiprocess_writes (worker TEXT NOT NULL, sequence INTEGER NOT NULL, PRIMARY KEY (worker, sequence))`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	type process struct {
		worker string
		cmd    *exec.Cmd
		output bytes.Buffer
	}
	processes := make([]process, multiprocessWorkers)
	for i := range processes {
		processes[i].worker = fmt.Sprintf("worker-%d", i)
		processes[i].cmd = exec.Command(os.Args[0], "-test.run=^TestOpenAcrossProcesses$")
		processes[i].cmd.Env = append(os.Environ(), multiprocessWorkerEnv+"="+processes[i].worker, multiprocessDBEnv+"="+path)
		processes[i].cmd.Stdout = &processes[i].output
		processes[i].cmd.Stderr = &processes[i].output
		if err := processes[i].cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	for i := range processes {
		process := &processes[i]
		if err := process.cmd.Wait(); err != nil {
			t.Fatalf("%s failed: %v\n%s", process.worker, err, process.output.String())
		}
	}

	db, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM multiprocess_writes`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if want := multiprocessWorkers * multiprocessRows; count != want {
		t.Fatalf("committed writes = %d, want %d", count, want)
	}
}

func writeFromSeparateProcess(t *testing.T, path, worker string) {
	t.Helper()
	if path == "" {
		t.Fatal("missing shared database path")
	}
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for sequence := range multiprocessRows {
		if err := db.WithinTx(context.Background(), func(tx *sql.Tx) error {
			if _, err := tx.Exec(`INSERT INTO multiprocess_writes(worker, sequence) VALUES (?, ?)`, worker, sequence); err != nil {
				return err
			}
			// Keep the write lock briefly so each process must honor SQLite's busy timeout.
			time.Sleep(25 * time.Millisecond)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
