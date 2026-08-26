// Package discovery resolves workspace identity without creating project files.
package discovery

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitMetadata struct {
	CanonicalCWD string
	Root         string
	CommonDir    string
	Branch       string
	Head         string
	Remote       string
	IsGit        bool
}

func DiscoverGit(cwd string) (GitMetadata, error) {
	canonical, err := canonicalPath(cwd)
	if err != nil {
		return GitMetadata{}, err
	}
	metadata := GitMetadata{CanonicalCWD: canonical}
	root, err := git(canonical, "rev-parse", "--show-toplevel")
	if err != nil {
		return metadata, nil
	}
	metadata.IsGit = true
	metadata.Root, err = canonicalPath(root)
	if err != nil {
		return metadata, fmt.Errorf("canonicalize git root: %w", err)
	}
	if common, err := git(canonical, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		metadata.CommonDir, _ = canonicalPath(common)
	}
	metadata.Branch, _ = git(canonical, "branch", "--show-current")
	metadata.Head, _ = git(canonical, "rev-parse", "HEAD")
	if remote, err := git(canonical, "remote", "get-url", "origin"); err == nil {
		metadata.Remote = NormalizeRemote(remote)
	}
	return metadata, nil
}

func NormalizeRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")
	for _, prefix := range []string{"ssh://", "https://", "http://"} {
		remote = strings.TrimPrefix(remote, prefix)
	}
	if strings.HasPrefix(remote, "git@") {
		remote = strings.TrimPrefix(remote, "git@")
		remote = strings.Replace(remote, ":", "/", 1)
	}
	return strings.ToLower(strings.Trim(remote, "/"))
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
