package fsutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// EnsureDir ensures that the directory exists, creating all parent directories.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// WriteFile writes data to a file, ensuring its parent directory exists.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, perm)
}

// WriteWithBackup writes data to a file. If the file already exists,
// it creates a backup with the suffix ".bak" (removing any existing backup file first to prevent errors)
// and then writes the new content.
func WriteWithBackup(path string, data []byte, perm os.FileMode) (backedUp bool, err error) {
	if Exists(path) {
		bakPath := path + ".bak"
		if Exists(bakPath) {
			if err := os.Remove(bakPath); err != nil {
				return false, err
			}
		}
		if err := os.Rename(path, bakPath); err != nil {
			return false, err
		}
		backedUp = true
	}

	if err := WriteFile(path, data, perm); err != nil {
		return false, err
	}

	return backedUp, nil
}
