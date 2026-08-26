// Package openspec resolves canonical artifact paths through the OpenSpec CLI.
package openspec

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
)

type Artifact struct {
	LogicalType string
	Path        string
	Status      string
}

type Runner func(context.Context, string, ...string) ([]byte, error)

func Enumerate(ctx context.Context, cwd, change string) ([]Artifact, error) {
	return enumerate(ctx, cwd, change, commandRunner(cwd))
}

func enumerate(ctx context.Context, cwd, change string, run Runner) ([]Artifact, error) {
	if change == "" {
		return nil, fmt.Errorf("OpenSpec change is required")
	}
	statusJSON, err := run(ctx, "openspec", "status", "--change", change, "--json")
	if err != nil {
		return nil, fmt.Errorf("OpenSpec status: %w", err)
	}
	instructionsJSON, err := run(ctx, "openspec", "instructions", "apply", "--change", change, "--json")
	if err != nil {
		return nil, fmt.Errorf("OpenSpec apply instructions: %w", err)
	}
	var status struct {
		ArtifactPaths map[string]struct {
			ExistingOutputPaths []string `json:"existingOutputPaths"`
		} `json:"artifactPaths"`
	}
	if err := json.Unmarshal(statusJSON, &status); err != nil {
		return nil, fmt.Errorf("decode OpenSpec status: %w", err)
	}
	var instructions struct {
		ContextFiles map[string][]string `json:"contextFiles"`
	}
	if err := json.Unmarshal(instructionsJSON, &instructions); err != nil {
		return nil, fmt.Errorf("decode OpenSpec instructions: %w", err)
	}
	seen := map[string]Artifact{}
	add := func(kind, path string) error {
		canonical, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		key := kind + "\x00" + canonical
		if _, exists := seen[key]; !exists {
			seen[key] = Artifact{LogicalType: kind, Path: canonical, Status: "observed"}
		}
		return nil
	}
	for kind, entry := range status.ArtifactPaths {
		for _, path := range entry.ExistingOutputPaths {
			if err := add(kind, path); err != nil {
				return nil, err
			}
		}
	}
	for kind, paths := range instructions.ContextFiles {
		for _, path := range paths {
			if err := add(kind, path); err != nil {
				return nil, err
			}
		}
	}
	artifacts := make([]Artifact, 0, len(seen))
	for _, artifact := range seen {
		artifacts = append(artifacts, artifact)
	}
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].LogicalType == artifacts[j].LogicalType {
			return artifacts[i].Path < artifacts[j].Path
		}
		return artifacts[i].LogicalType < artifacts[j].LogicalType
	})
	return artifacts, nil
}

func commandRunner(cwd string) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = cwd
		return cmd.Output()
	}
}
