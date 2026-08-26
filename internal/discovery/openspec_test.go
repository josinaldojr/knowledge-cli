package discovery

import (
	"context"
	"errors"
	"testing"
)

func TestResolveOpenSpecUsesCLIResolvedArtifactPaths(t *testing.T) {
	resolution, err := resolveOpenSpec(context.Background(), "/workspace", "change", func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"planningHome":{"root":"/planning"},"changeRoot":"/planning/changes/change","artifactPaths":{"proposal":{"existingOutputPaths":["/planning/changes/change/proposal.md"]},"specs":{"existingOutputPaths":["/planning/changes/change/specs/api.md"]}}}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.PlanningRoot != "/planning" || resolution.ChangeRoot != "/planning/changes/change" {
		t.Fatalf("resolution = %#v", resolution)
	}
	if got := resolution.Artifacts["specs"][0]; got != "/planning/changes/change/specs/api.md" {
		t.Fatalf("artifact = %q", got)
	}
}

func TestResolveOpenSpecReturnsRunnerErrors(t *testing.T) {
	_, err := resolveOpenSpec(context.Background(), "/workspace", "change", func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("not installed") })
	if err == nil {
		t.Fatal("expected error")
	}
}
