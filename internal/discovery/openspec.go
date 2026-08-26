package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type OpenSpecResolution struct {
	PlanningRoot string
	ChangeRoot   string
	Artifacts    map[string][]string
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

func ResolveOpenSpec(ctx context.Context, cwd, change string) (OpenSpecResolution, error) {
	return resolveOpenSpec(ctx, cwd, change, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = cwd
		return cmd.Output()
	})
}

func resolveOpenSpec(ctx context.Context, cwd, change string, run commandRunner) (OpenSpecResolution, error) {
	if change == "" {
		return OpenSpecResolution{}, fmt.Errorf("OpenSpec change is required")
	}
	output, err := run(ctx, "openspec", "status", "--change", change, "--json")
	if err != nil {
		return OpenSpecResolution{}, fmt.Errorf("resolve OpenSpec status: %w", err)
	}
	var status struct {
		ChangeRoot   string `json:"changeRoot"`
		PlanningHome struct {
			Root string `json:"root"`
		} `json:"planningHome"`
		ArtifactPaths map[string]struct {
			ExistingOutputPaths []string `json:"existingOutputPaths"`
		} `json:"artifactPaths"`
	}
	if err := json.Unmarshal(output, &status); err != nil {
		return OpenSpecResolution{}, fmt.Errorf("decode OpenSpec status: %w", err)
	}
	if status.ChangeRoot == "" || status.PlanningHome.Root == "" {
		return OpenSpecResolution{}, fmt.Errorf("OpenSpec status omitted planning or change root")
	}
	artifacts := make(map[string][]string, len(status.ArtifactPaths))
	for kind, artifact := range status.ArtifactPaths {
		artifacts[kind] = append([]string(nil), artifact.ExistingOutputPaths...)
	}
	return OpenSpecResolution{PlanningRoot: status.PlanningHome.Root, ChangeRoot: status.ChangeRoot, Artifacts: artifacts}, nil
}
