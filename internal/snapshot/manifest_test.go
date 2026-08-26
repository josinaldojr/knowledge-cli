package snapshot

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kv/internal/openspec"
)

func TestBuildManifestUsesSHA256AndCanonicalPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proposal.md")
	if err := os.WriteFile(path, []byte("scope"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, contents, err := Build(openspec.Artifact{LogicalType: "proposal", Path: path, Status: "observed"}, "abc", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ContentHash != "5f161c9149882e0e10124bc5dd5c11f0fbe8ec452edd52bcec76b01e9252cb33" || string(contents) != "scope" {
		t.Fatalf("manifest = %#v, content = %q", manifest, contents)
	}
}

func TestBuildManifestRejectsSymlinkedArtifacts(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "outside.md")
	link := filepath.Join(dir, "proposal.md")
	if err := os.WriteFile(target, []byte("scope"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Build(openspec.Artifact{LogicalType: "proposal", Path: link}, "", time.Now()); err == nil {
		t.Fatal("symlinked artifact was accepted")
	}
}

func TestBuildManifestRemainsInternallyConsistentDuringConcurrentUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proposal.md")
	if err := os.WriteFile(path, []byte("initial"), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for {
			select {
			case <-done:
				return
			default:
				_ = os.WriteFile(path, []byte("updated artifact content"), 0600)
			}
		}
	}()
	for range 25 {
		manifest, contents, err := Build(openspec.Artifact{LogicalType: "proposal", Path: path}, "", time.Now())
		if err != nil {
			close(done)
			writer.Wait()
			t.Fatal(err)
		}
		if manifest.Size != int64(len(contents)) {
			close(done)
			writer.Wait()
			t.Fatalf("manifest size = %d, content size = %d", manifest.Size, len(contents))
		}
	}
	close(done)
	writer.Wait()
}
