package runner

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/session"
)

func TestRunTask(t *testing.T) {
	// Mock runCommandOverride
	oldOverride := runCommandOverride
	runCommandOverride = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}
	defer func() { runCommandOverride = oldOverride }()

	tmpDir, err := ioutil.TempDir("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	workflowSlug := "test-flow"
	taskID := "001-setup"

	// Create context folder and context pack
	workflowDir := filepath.Join(tmpDir, ".kv", "workflows", workflowSlug)
	contextDir := filepath.Join(workflowDir, "context")
	err = os.MkdirAll(contextDir, 0755)
	if err != nil {
		t.Fatalf("failed to create context dir: %v", err)
	}

	contextPackPath := filepath.Join(contextDir, taskID+".context.md")
	err = ioutil.WriteFile(contextPackPath, []byte("my context pack"), 0644)
	if err != nil {
		t.Fatalf("failed to write context pack: %v", err)
	}

	// Run with unsupported runner
	err = RunTask(tmpDir, workflowSlug, taskID, "unsupported")
	if err == nil {
		t.Errorf("expected error for unsupported runner, got nil")
	}

	// Run with opencode
	err = RunTask(tmpDir, workflowSlug, taskID, "opencode")
	if err != nil {
		t.Fatalf("RunTask opencode failed: %v", err)
	}

	// Verify .opencode/context.md is generated
	opencodeFile := filepath.Join(tmpDir, ".opencode", "context.md")
	if _, err := os.Stat(opencodeFile); os.IsNotExist(err) {
		t.Fatalf(".opencode/context.md not created")
	}
}

func TestOpenCodeRunner_Run(t *testing.T) {
	// Initialize test git repo (necessary for boundary.ValidateSession)
	tmpDir, err := ioutil.TempDir("", "runner-test-opencode-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if eval, err := filepath.EvalSymlinks(tmpDir); err == nil {
		tmpDir = eval
	}

	runGitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run git %v: %v", args, err)
		}
	}
	runGitCmd("init")
	runGitCmd("config", "user.name", "Test")
	runGitCmd("config", "user.email", "test@example.com")

	// Create a dummy commit so git works normally
	dummyFile := filepath.Join(tmpDir, "dummy.txt")
	err = ioutil.WriteFile(dummyFile, []byte("dummy"), 0644)
	if err != nil {
		t.Fatalf("failed to write dummy: %v", err)
	}
	runGitCmd("add", "dummy.txt")
	runGitCmd("commit", "-m", "initial commit")

	// Set up a session
	sessID := "sess-opencode-test"
	sessDir := filepath.Join(tmpDir, ".kv", "sessions", sessID)
	err = os.MkdirAll(sessDir, 0755)
	if err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	// Create prompt path
	promptPath := filepath.Join(sessDir, "opencode.md")
	err = ioutil.WriteFile(promptPath, []byte("run agent prompt"), 0644)
	if err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}

	// Session config
	sess := &session.Session{
		ID:        sessID,
		Goal:      "Test OpenCode execution",
		CreatedAt: time.Now(),
		Status:    "active",
		Boundary: session.Boundary{
			AllowedPaths:  []string{tmpDir},
			WritablePaths: []string{tmpDir},
		},
		Agent: session.AgentContract{
			Provider: "opencode",
		},
	}

	// Mock runCommandOverride
	oldOverride := runCommandOverride
	var calledName string
	var calledArgs []string
	runCommandOverride = func(name string, arg ...string) *exec.Cmd {
		calledName = name
		calledArgs = arg
		return exec.Command("true")
	}
	defer func() { runCommandOverride = oldOverride }()

	runner := &OpenCodeRunner{WorkspaceDir: tmpDir}
	res, err := runner.Run(sess, promptPath, false)
	if err != nil {
		t.Fatalf("runner.Run failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success to be true")
	}

	if calledName != "opencode" {
		t.Errorf("expected command 'opencode', got '%s'", calledName)
	}

	if len(calledArgs) < 2 || calledArgs[0] != "run" || calledArgs[1] != "run agent prompt" {
		t.Errorf("expected args ['run', 'run agent prompt'], got %v", calledArgs)
	}
}
