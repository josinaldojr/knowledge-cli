// Package store provides the global, project-independent KV persistence paths.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	dataHomeEnv  = "KV_DATA_HOME"
	cacheHomeEnv = "KV_CACHE_HOME"
)

type Paths struct {
	DataDir      string
	CacheDir     string
	SnapshotDir  string
	SpoolDir     string
	DatabasePath string
}

func ResolvePaths() (Paths, error) {
	return resolvePaths(runtime.GOOS, os.Getenv, os.UserHomeDir)
}

func resolvePaths(goos string, getenv func(string) string, homeDir func() (string, error)) (Paths, error) {
	home, err := homeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home: %w", err)
	}
	data := getenv(dataHomeEnv)
	if data == "" {
		switch goos {
		case "windows":
			data = getenv("LOCALAPPDATA")
			if data == "" {
				data = filepath.Join(home, "AppData", "Local")
			}
			data = filepath.Join(data, "kv")
		case "darwin":
			data = filepath.Join(home, "Library", "Application Support", "kv")
		default:
			base := getenv("XDG_DATA_HOME")
			if base == "" {
				base = filepath.Join(home, ".local", "share")
			}
			data = filepath.Join(base, "kv")
		}
	}
	cache := getenv(cacheHomeEnv)
	if cache == "" {
		switch goos {
		case "windows":
			cache = filepath.Join(data, "cache")
		case "darwin":
			cache = filepath.Join(home, "Library", "Caches", "kv")
		default:
			base := getenv("XDG_CACHE_HOME")
			if base == "" {
				base = filepath.Join(home, ".cache")
			}
			cache = filepath.Join(base, "kv")
		}
	}
	data, err = filepath.Abs(data)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve data directory: %w", err)
	}
	cache, err = filepath.Abs(cache)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve cache directory: %w", err)
	}
	return Paths{DataDir: data, CacheDir: cache, SnapshotDir: filepath.Join(data, "snapshots"), SpoolDir: filepath.Join(data, "spool"), DatabasePath: filepath.Join(data, "kv.db")}, nil
}
