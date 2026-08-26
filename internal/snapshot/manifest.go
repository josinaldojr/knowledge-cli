// Package snapshot creates content-addressed OpenSpec artifact manifests.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kv/internal/openspec"
)

const ParserVersion = "openspec-manifest-v1"

type Manifest struct {
	LogicalType   string    `json:"logical_type"`
	CanonicalPath string    `json:"canonical_path"`
	ContentHash   string    `json:"content_hash"`
	Size          int64     `json:"size"`
	Status        string    `json:"status"`
	ParserVersion string    `json:"parser_version"`
	CapturedAt    time.Time `json:"captured_at"`
	GitRevision   string    `json:"git_revision,omitempty"`
}

func Build(artifact openspec.Artifact, gitRevision string, capturedAt time.Time) (Manifest, []byte, error) {
	entry, err := os.Lstat(artifact.Path)
	if err != nil {
		return Manifest{}, nil, err
	}
	if entry.Mode()&os.ModeSymlink != 0 {
		return Manifest{}, nil, fmt.Errorf("artifact symlinks are not eligible for snapshots: %s", artifact.Path)
	}
	canonical, err := filepath.EvalSymlinks(artifact.Path)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("canonicalize artifact: %w", err)
	}
	contents, err := os.ReadFile(canonical)
	if err != nil {
		return Manifest{}, nil, err
	}
	digest := sha256.Sum256(contents)
	return Manifest{LogicalType: artifact.LogicalType, CanonicalPath: canonical, ContentHash: hex.EncodeToString(digest[:]), Size: int64(len(contents)), Status: artifact.Status, ParserVersion: ParserVersion, CapturedAt: capturedAt.UTC(), GitRevision: gitRevision}, contents, nil
}
