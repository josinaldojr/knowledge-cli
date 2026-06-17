package fsutil

import (
	"os"
	"path/filepath"
)

// Exists checks if a file or directory exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir checks if the path exists and is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsFile checks if the path exists and is a regular file.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// ResolveAbs resolves a path to an absolute representation.
func ResolveAbs(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// ResolveRel returns a relative path from base to target if possible,
// or fallback to absolute path if not possible (e.g. different drives on Windows).
func ResolveRel(base, target string) string {
	absBase, err := ResolveAbs(base)
	if err != nil {
		return target
	}
	absTarget, err := ResolveAbs(target)
	if err != nil {
		return target
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return absTarget
	}

	// Clean and standardise separation
	return filepath.Clean(rel)
}
