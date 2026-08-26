package discovery

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDiscoverGitAndNormalizeRemote(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}, {"commit", "--allow-empty", "-m", "initial"}, {"remote", "add", "origin", "git@github.com:Example/Repo.git"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
	}
	metadata, err := DiscoverGit(dir)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !metadata.IsGit || metadata.Root != canonical || metadata.Head == "" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if metadata.Remote != "github.com/example/repo" {
		t.Fatalf("remote = %q", metadata.Remote)
	}
	if filepath.Clean(metadata.CanonicalCWD) != canonical {
		t.Fatalf("cwd = %q", metadata.CanonicalCWD)
	}
}

func TestDiscoverGitOutsideRepository(t *testing.T) {
	metadata, err := DiscoverGit(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if metadata.IsGit {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestNormalizeRemote(t *testing.T) {
	for input, want := range map[string]string{"https://github.com/Example/Repo.git": "github.com/example/repo", "ssh://git@github.com/Example/Repo.git": "github.com/example/repo"} {
		if got := NormalizeRemote(input); got != want {
			t.Errorf("NormalizeRemote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDiscoverGitWorktreeAndMovedDirectory(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	for _, args := range [][]string{{"init", repo}, {"-C", repo, "config", "user.email", "test@example.com"}, {"-C", repo, "config", "user.name", "Test"}, {"-C", repo, "commit", "--allow-empty", "-m", "initial"}} {
		if err := exec.Command("git", args...).Run(); err != nil {
			t.Fatal(err)
		}
	}
	worktree := filepath.Join(root, "linked")
	if err := exec.Command("git", "-C", repo, "worktree", "add", "-b", "linked-branch", worktree).Run(); err != nil {
		t.Fatal(err)
	}
	linked, err := DiscoverGit(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if !linked.IsGit || linked.Root == linked.CommonDir {
		t.Fatalf("linked worktree metadata = %#v", linked)
	}
	moved := filepath.Join(root, "moved")
	if err := os.Rename(repo, moved); err != nil {
		t.Fatal(err)
	}
	metadata, err := DiscoverGit(moved)
	if err != nil || !metadata.IsGit {
		t.Fatalf("moved repository = %#v, %v", metadata, err)
	}
}
