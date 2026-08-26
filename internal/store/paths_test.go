package store

import (
	"path/filepath"
	"testing"
)

func TestResolvePathsUsesOverrides(t *testing.T) {
	paths, err := resolvePaths("linux", func(key string) string {
		if key == dataHomeEnv {
			return "/tmp/kv-data"
		}
		if key == cacheHomeEnv {
			return "/tmp/kv-cache"
		}
		return ""
	}, func() (string, error) { return "/home/test", nil })
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != "/tmp/kv-data" || paths.CacheDir != "/tmp/kv-cache" {
		t.Fatalf("unexpected paths: %#v", paths)
	}
	if paths.SnapshotDir != filepath.Join("/tmp/kv-data", "snapshots") {
		t.Fatal("snapshot path is not under data path")
	}
}

func TestResolvePathsUsesPlatformDefaults(t *testing.T) {
	paths, err := resolvePaths("darwin", func(string) string { return "" }, func() (string, error) { return "/Users/test", nil })
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != "/Users/test/Library/Application Support/kv" {
		t.Fatalf("data path = %s", paths.DataDir)
	}
	if paths.CacheDir != "/Users/test/Library/Caches/kv" {
		t.Fatalf("cache path = %s", paths.CacheDir)
	}
}
