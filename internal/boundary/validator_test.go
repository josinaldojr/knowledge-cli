package boundary

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/session"

	"gopkg.in/yaml.v3"
)

func setupTestGitRepo(t *testing.T) (string, func()) {
	tmpDir, err := ioutil.TempDir("", "kv-boundary-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run git %v: %v", args, err)
		}
	}

	runCmd("init")
	runCmd("config", "user.name", "Test")
	runCmd("config", "user.email", "test@example.com")

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestValidateSession(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Resolve symlinks of repoDir for macOS temp paths compatibility
	if eval, err := filepath.EvalSymlinks(repoDir); err == nil {
		repoDir = eval
	}

	// 1. Create a session file
	sessionID := "sess-test"
	sessionDir := filepath.Join(repoDir, ".kv", "sessions", sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	sess := &session.Session{
		ID:           sessionID,
		Goal:         "Test boundary validation",
		SelectedApps: []string{"app1"},
		CreatedAt:    time.Now(),
		Status:       "active",
		Boundary: session.Boundary{
			AllowedPaths: []string{
				filepath.Join(repoDir, "allowed-dir"),
			},
			DeniedPaths: []string{
				filepath.Join(repoDir, "allowed-dir/blocked.txt"),
				"**/*.secret", // relative glob denied pattern
			},
		},
	}

	// Save session to .yaml
	data, err := yaml.Marshal(sess)
	if err != nil {
		t.Fatalf("failed to marshal session: %v", err)
	}
	sessionFile := filepath.Join(sessionDir, "session.yaml")
	if err := ioutil.WriteFile(sessionFile, data, 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	// 2. Test nonexistent session loading
	_, loadErr := session.LoadSession(repoDir, "nonexistent-sess")
	if loadErr == nil {
		t.Errorf("expected error loading nonexistent session, got nil")
	}

	// 3. Test validation when no files are changed
	report, err := ValidateSession(repoDir, sess, true)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if len(report.Allowed) != 0 || len(report.Denied) != 0 || len(report.OutOfScope) != 0 {
		t.Errorf("expected empty report for no changes, got allowed: %v, denied: %v, outOfScope: %v",
			report.Allowed, report.Denied, report.OutOfScope)
	}

	// 4. Setup some files in the repo
	// Let's create a tracked file first to have a base
	baseFile := filepath.Join(repoDir, "base.txt")
	if err := ioutil.WriteFile(baseFile, []byte("base"), 0644); err != nil {
		t.Fatalf("failed to write base file: %v", err)
	}
	
	// Commit it
	runGitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed git %v: %v", args, err)
		}
	}
	runGitCmd("add", "base.txt")
	runGitCmd("commit", "-m", "initial commit")

	// Now make changes:
	// A) Allowed changed file (unstaged)
	allowedDir := filepath.Join(repoDir, "allowed-dir")
	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("failed to create allowed dir: %v", err)
	}
	allowedFile := filepath.Join(allowedDir, "file.txt")
	if err := ioutil.WriteFile(allowedFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write allowed file: %v", err)
	}
	// Let's track allowed file as unstaged/staged by staging it first
	runGitCmd("add", "allowed-dir/file.txt")
	// Make a modification to make it unstaged but modified
	if err := ioutil.WriteFile(allowedFile, []byte("hello modified"), 0644); err != nil {
		t.Fatalf("failed to write allowed file: %v", err)
	}

	// B) Staged changed file
	stagedFile := filepath.Join(allowedDir, "staged.txt")
	if err := ioutil.WriteFile(stagedFile, []byte("staged"), 0644); err != nil {
		t.Fatalf("failed to write staged file: %v", err)
	}
	runGitCmd("add", "allowed-dir/staged.txt")

	// C) Denied changed file (explicitly blocked)
	deniedFile := filepath.Join(allowedDir, "blocked.txt")
	if err := ioutil.WriteFile(deniedFile, []byte("blocked"), 0644); err != nil {
		t.Fatalf("failed to write blocked file: %v", err)
	}
	runGitCmd("add", "allowed-dir/blocked.txt")

	// D) Denied changed file by glob pattern (**/*.secret)
	secretFile := filepath.Join(allowedDir, "some.secret")
	if err := ioutil.WriteFile(secretFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}
	runGitCmd("add", "allowed-dir/some.secret")

	// E) Out of scope changed file (unstaged)
	outOfScopeFile := filepath.Join(repoDir, "out-of-scope.txt")
	if err := ioutil.WriteFile(outOfScopeFile, []byte("out of scope"), 0644); err != nil {
		t.Fatalf("failed to write out of scope file: %v", err)
	}
	runGitCmd("add", "out-of-scope.txt")

	// F) Untracked file
	untrackedFile := filepath.Join(allowedDir, "untracked.txt")
	if err := ioutil.WriteFile(untrackedFile, []byte("untracked"), 0644); err != nil {
		t.Fatalf("failed to write untracked file: %v", err)
	}

	// Run validation WITH untracked files
	report, err = ValidateSession(repoDir, sess, true)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}

	// Helper to check if file is in validation list
	containsPath := func(list []FileValidation, path string) bool {
		absPath := filepath.Clean(path)
		if eval, err := filepath.EvalSymlinks(absPath); err == nil {
			absPath = eval
		}
		for _, f := range list {
			fPath := filepath.Clean(f.Path)
			if eval, err := filepath.EvalSymlinks(fPath); err == nil {
				fPath = eval
			}
			if fPath == absPath {
				return true
			}
		}
		return false
	}

	// Assertions:
	// - Allowed: allowed-dir/file.txt, allowed-dir/staged.txt, allowed-dir/untracked.txt
	if !containsPath(report.Allowed, allowedFile) {
		t.Errorf("expected report to allow %s", allowedFile)
	}
	if !containsPath(report.Allowed, stagedFile) {
		t.Errorf("expected report to allow %s", stagedFile)
	}
	if !containsPath(report.Allowed, untrackedFile) {
		t.Errorf("expected report to allow untracked file %s", untrackedFile)
	}

	// - Denied: allowed-dir/blocked.txt, allowed-dir/some.secret
	if !containsPath(report.Denied, deniedFile) {
		t.Errorf("expected report to deny %s", deniedFile)
	}
	if !containsPath(report.Denied, secretFile) {
		t.Errorf("expected report to deny secret file %s", secretFile)
	}

	// - Out of scope: out-of-scope.txt
	if !containsPath(report.OutOfScope, outOfScopeFile) {
		t.Errorf("expected report to mark out of scope: %s", outOfScopeFile)
	}

	if report.IsValid() {
		t.Errorf("expected report to be invalid")
	}

	// Run validation WITHOUT untracked files
	reportNoUntracked, err := ValidateSession(repoDir, sess, false)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if containsPath(reportNoUntracked.Allowed, untrackedFile) {
		t.Errorf("expected report without untracked files to NOT contain %s", untrackedFile)
	}
}
