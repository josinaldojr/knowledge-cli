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

// ScanWorkspaceApps walks the workspace directory and returns detected applications.
func ScanWorkspaceApps(workspaceDir string) ([]App, error) {
	var apps []App

	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // ignore errors
		}
		if !info.IsDir() {
			return nil
		}

		name := info.Name()
		if name == ".git" || name == "node_modules" || name == ".kv" || name == ".opencode" || name == "dist" || name == "build" || name == "target" || name == "vendor" {
			return filepath.SkipDir
		}

		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil
		}

		stack := ""
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			fname := f.Name()
			switch fname {
			case "go.mod":
				stack = "Go"
			case "package.json":
				stack = "Node/Vite"
			case "pom.xml", "build.gradle":
				stack = "Java"
			case "requirements.txt", "pyproject.toml":
				stack = "Python"
			case "Dockerfile":
				if stack == "" {
					stack = "Docker"
				}
			}
		}

		if stack != "" {
			relPath, err := filepath.Rel(workspaceDir, path)
			if err != nil {
				return nil
			}

			cleanRel := filepath.Clean(relPath)
			if cleanRel == "." {
				cleanRel = "./"
			} else {
				if !strings.HasPrefix(cleanRel, "./") && !strings.HasPrefix(cleanRel, "../") {
					cleanRel = "./" + cleanRel
				}
			}

			id := filepath.Base(path)
			if id == "." || id == "/" || id == workspaceDir {
				id = "root-app"
			}

			apps = append(apps, App{
				ID:    id,
				Name:  id,
				Path:  cleanRel,
				Type:  "service",
				Stack: stack,
			})
		}

		return nil
	})

	return apps, err
}
