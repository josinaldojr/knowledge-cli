package openspec

import (
	"context"
	"testing"
)

func TestEnumerateUsesResolvedStatusAndInstructionPaths(t *testing.T) {
	calls := 0
	artifacts, err := enumerate(context.Background(), "/workspace", "change", func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls++
		if args[0] == "status" {
			return []byte(`{"artifactPaths":{"proposal":{"existingOutputPaths":["/planning/proposal.md"]},"archive":{"existingOutputPaths":["/planning/archive/change.md"]}}}`), nil
		}
		return []byte(`{"contextFiles":{"proposal":["/planning/proposal.md"],"specs":["/planning/specs/a.md"]}}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(artifacts) != 3 {
		t.Fatalf("calls = %d, artifacts = %#v", calls, artifacts)
	}
	if artifacts[0].LogicalType != "archive" || artifacts[2].LogicalType != "specs" {
		t.Fatalf("artifacts = %#v", artifacts)
	}
}
