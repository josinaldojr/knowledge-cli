package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/hook"
	"kv/internal/memory"
	"kv/internal/store"
)

func BenchmarkBeforeHookRetrieval(b *testing.B) {
	ctx := context.Background()
	databasePath := filepath.Join(b.TempDir(), "kv.db")
	database, err := store.Open(ctx, databasePath)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = database.Close() })
	if err := database.Migrate(ctx, databasePath); err != nil {
		b.Fatal(err)
	}
	repository := store.NewRepository(database)
	service := NewHookApplicationService(NewSessionService(repository), repository).WithMemoryRetriever(memory.NewRetriever(repository))
	workspace := b.TempDir()
	b.ReportAllocs()
	for index := 0; b.Loop(); index++ {
		envelope := hook.HookEnvelope{
			SchemaVersion:  hook.SchemaVersion,
			EventID:        fmt.Sprintf("before-%d", index),
			IdempotencyKey: fmt.Sprintf("before-%d", index),
			Event:          hook.EventBeforeApply,
			OccurredAt:     time.Now(),
			Provider:       hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "benchmark"},
			Workspace:      hook.WorkspaceIdentity{CWD: workspace},
			Subject:        hook.HookSubject{ChangeID: "benchmark", Operation: hook.Operation{ID: fmt.Sprintf("operation-%d", index), Kind: "apply"}},
		}
		if _, err := service.Handle(ctx, envelope); err != nil {
			b.Fatal(err)
		}
	}
}
