package workspace

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"kv/internal/fsutil"
)

const MarkerFilename = ".knowledge-vault"
const BackupSuffix = ".bak"

// ReadMarker reads the raw path stored in .knowledge-vault in the given directory.
func ReadMarker(dir string) (string, error) {
	markerPath := filepath.Join(dir, MarkerFilename)
	data, err := ioutil.ReadFile(markerPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteMarker writes the vault path to .knowledge-vault in the given directory.
// If the marker already exists, it backs it up to .knowledge-vault.bak before overwriting.
func WriteMarker(dir string, vaultPath string) (backedUp bool, err error) {
	markerPath := filepath.Join(dir, MarkerFilename)
	data := []byte(vaultPath)
	return fsutil.WriteWithBackup(markerPath, data, 0644)
}

// ResolvePathSafe resolves the value relative to the base directory if it's relative,
// otherwise returns the absolute path if it is already rooted/absolute.
func ResolvePathSafe(base, value string) (string, error) {
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	// On Windows, filepath.IsAbs might not catch some cases, but filepath.Join handles both.
	// We'll clean it up.
	absBase, err := fsutil.ResolveAbs(base)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(absBase, value)), nil
}
