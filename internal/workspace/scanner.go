package workspace

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// ScanRepo returns a list of relative file paths in the workspace, skipping ignored folders.
func ScanRepo(workspaceDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		name := info.Name()
		if info.IsDir() {
			if name == ".git" || name == "node_modules" || name == ".kv" || name == ".opencode" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(workspaceDir, path)
		if err != nil {
			return nil
		}
		files = append(files, relPath)
		return nil
	})
	return files, err
}

// ReadRepoFile reads the content of a file in the workspace safely.
func ReadRepoFile(workspaceDir, relPath string) (string, error) {
	// Clean the path to avoid directory traversal
	cleanPath := filepath.Clean(relPath)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("path traversal attempt detected or absolute path used: %s", relPath)
	}

	fullPath := filepath.Join(workspaceDir, cleanPath)
	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
