package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func BenchmarkSQLiteContention(b *testing.B) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(b.TempDir(), "kv.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = database.Close() })
	if _, err := database.DB.Exec(`CREATE TABLE benchmark_writes (value INTEGER NOT NULL)`); err != nil {
		b.Fatal(err)
	}
	b.SetParallelism(4)
	b.ReportAllocs()
	b.RunParallel(func(worker *testing.PB) {
		for worker.Next() {
			if err := database.WithinTx(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `INSERT INTO benchmark_writes(value) VALUES (1)`)
				return err
			}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
