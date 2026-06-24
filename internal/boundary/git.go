package boundary

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetGitChanges returns the absolute paths of all changed files (staged, unstaged, and optionally untracked)
// relative to the workspace/git directory. It also returns the absolute path of the Git top-level root.
func GetGitChanges(dir string, includeUntracked bool) (string, []string, error) {
	// First, get the git top-level directory
	gitRoot, err := runGitCommand(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil, fmt.Errorf("failed to get git root (is this a git repository?): %w", err)
	}
	gitRoot, err = filepath.EvalSymlinks(gitRoot)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve symlinks for git root: %w", err)
	}

	// 1. Get unstaged changes
	unstagedStr, err := runGitCommand(dir, "diff", "--name-only")
	if err != nil {
		return "", nil, fmt.Errorf("failed to get unstaged changes: %w", err)
	}

	// 2. Get staged changes
	stagedStr, err := runGitCommand(dir, "diff", "--cached", "--name-only")
	if err != nil {
		return "", nil, fmt.Errorf("failed to get staged changes: %w", err)
	}

	seen := make(map[string]bool)
	var files []string

	addFile := func(relPath string) {
		relPath = strings.TrimSpace(relPath)
		if relPath == "" {
			return
		}
		// Git paths are relative to the git root, so make them absolute
		absPath := filepath.Clean(filepath.Join(gitRoot, relPath))
		if !seen[absPath] {
			seen[absPath] = true
			files = append(files, absPath)
		}
	}

	// Parse unstaged changes
	for _, f := range strings.Split(unstagedStr, "\n") {
		addFile(f)
	}

	// Parse staged changes
	for _, f := range strings.Split(stagedStr, "\n") {
		addFile(f)
	}

	// 3. Get untracked changes if requested
	if includeUntracked {
		untrackedStr, err := runGitCommand(dir, "ls-files", "--others", "--exclude-standard")
		if err != nil {
			return "", nil, fmt.Errorf("failed to get untracked files: %w", err)
		}
		for _, f := range strings.Split(untrackedStr, "\n") {
			addFile(f)
		}
	}

	return gitRoot, files, nil
}

func runGitCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
