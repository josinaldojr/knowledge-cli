package lifecycle

import (
	"testing"

	"kv/internal/memory"
)

func TestToMemoryContextPreservesEvidenceAndEmptyResults(t *testing.T) {
	if contexts := toMemoryContext(nil); len(contexts) != 0 {
		t.Fatalf("empty contexts = %#v", contexts)
	}
	contexts := toMemoryContext([]memory.Result{{ID: "memory", Kind: "decision", Status: "current", Summary: "Use WAL", Score: 9, Current: true, ChangeKey: "change", ArtifactPath: "/repo/design.md", RevisionID: "revision"}})
	if len(contexts) != 1 || contexts[0].Source.ChangeID != "change" || contexts[0].Source.ArtifactID != "/repo/design.md" || contexts[0].Source.RevisionID != "revision" {
		t.Fatalf("contexts = %#v", contexts)
	}
}
