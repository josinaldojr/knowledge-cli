package runner

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kv/internal/session"
)

func TestRunTask(t *testing.T) {
	// Mock RunCommandOverride
	oldOverride := RunCommandOverride
	RunCommandOverride = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}
	defer func() { RunCommandOverride = oldOverride }()

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
	err = RunTask(tmpDir, workflowSlug, taskID, "unsupported", "", "")
	if err == nil {
		t.Errorf("expected error for unsupported runner, got nil")
	}

	// Run with opencode
	err = RunTask(tmpDir, workflowSlug, taskID, "opencode", "", "")
	if err != nil {
		t.Fatalf("RunTask opencode failed: %v", err)
	}

	// Verify .opencode/context.md is generated
	opencodeFile := filepath.Join(tmpDir, ".opencode", "context.md")
	if _, err := os.Stat(opencodeFile); os.IsNotExist(err) {
		t.Fatalf(".opencode/context.md not created")
	}
}

func TestRunTask_ClaudeCodeAndCodex(t *testing.T) {
	oldOverride := RunCommandOverride
	var calledName string
	var calledArgs []string
	RunCommandOverride = func(name string, arg ...string) *exec.Cmd {
		calledName = name
		calledArgs = arg
		return exec.Command("true")
	}
	defer func() { RunCommandOverride = oldOverride }()

	tmpDir, err := ioutil.TempDir("", "runner-test-claude-codex-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	workflowSlug := "test-flow"
	taskID := "001-setup"
	contextDir := filepath.Join(tmpDir, ".kv", "workflows", workflowSlug, "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		t.Fatalf("failed to create context dir: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(contextDir, taskID+".context.md"), []byte("my context pack"), 0644); err != nil {
		t.Fatalf("failed to write context pack: %v", err)
	}

	// claude-code runner
	if err := RunTask(tmpDir, workflowSlug, taskID, "claude-code", "", "sonnet"); err != nil {
		t.Fatalf("RunTask claude-code failed: %v", err)
	}
	if calledName != "claude" {
		t.Errorf("expected command 'claude', got '%s'", calledName)
	}
	if len(calledArgs) < 3 || calledArgs[0] != "-p" || calledArgs[2] != "--model" || calledArgs[3] != "sonnet" {
		t.Errorf("unexpected claude args: %v", calledArgs)
	}

	// codex runner
	if err := RunTask(tmpDir, workflowSlug, taskID, "codex", "", ""); err != nil {
		t.Fatalf("RunTask codex failed: %v", err)
	}
	if calledName != "codex" {
		t.Errorf("expected command 'codex', got '%s'", calledName)
	}
	if len(calledArgs) < 2 || calledArgs[0] != "exec" {
		t.Errorf("unexpected codex args: %v", calledArgs)
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

	// Create dummy opencode.json in workspace to validate agent
	opencodeDir := filepath.Join(tmpDir, ".opencode")
	err = os.MkdirAll(opencodeDir, 0755)
	if err != nil {
		t.Fatalf("failed to create .opencode dir: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(opencodeDir, "opencode.json"), []byte(`{"agents":{"backend":{}}}`), 0644)
	if err != nil {
		t.Fatalf("failed to write opencode.json: %v", err)
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

	// Mock RunCommandOverride
	oldOverride := RunCommandOverride
	var calledName string
	var calledArgs []string
	RunCommandOverride = func(name string, arg ...string) *exec.Cmd {
		calledName = name
		calledArgs = arg
		return exec.Command("true")
	}
	defer func() { RunCommandOverride = oldOverride }()

	runner := &OpenCodeRunner{WorkspaceDir: tmpDir}
	res, err := runner.Run(sess, promptPath, "backend", false)
	if err != nil {
		t.Fatalf("runner.Run failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success to be true")
	}

	if calledName != "opencode" {
		t.Errorf("expected command 'opencode', got '%s'", calledName)
	}

	if len(calledArgs) < 4 || calledArgs[0] != "run" || calledArgs[1] != "--agent" || calledArgs[2] != "backend" || calledArgs[3] != "run agent prompt" {
		t.Errorf("expected args ['run', '--agent', 'backend', 'run agent prompt'], got %v", calledArgs)
	}
}

func TestClaudeCodeAndCodexRunner_Run(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "runner-test-claude-codex-run-*")
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
	if err := ioutil.WriteFile(filepath.Join(tmpDir, "dummy.txt"), []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to write dummy: %v", err)
	}
	runGitCmd("add", "dummy.txt")
	runGitCmd("commit", "-m", "initial commit")

	sessID := "sess-provider-test"
	sessDir := filepath.Join(tmpDir, ".kv", "sessions", sessID)
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}
	promptPath := filepath.Join(sessDir, "opencode.md")
	if err := ioutil.WriteFile(promptPath, []byte("run agent prompt"), 0644); err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}

	sess := &session.Session{
		ID:        sessID,
		Goal:      "Test provider dispatch",
		CreatedAt: time.Now(),
		Status:    "active",
		Boundary: session.Boundary{
			AllowedPaths:  []string{tmpDir},
			WritablePaths: []string{tmpDir},
		},
	}

	oldOverride := RunCommandOverride
	var calledName string
	var calledArgs []string
	RunCommandOverride = func(name string, arg ...string) *exec.Cmd {
		calledName = name
		calledArgs = arg
		return exec.Command("true")
	}
	defer func() { RunCommandOverride = oldOverride }()

	claudeRunner := &ClaudeCodeRunner{WorkspaceDir: tmpDir, Model: "opus"}
	res, err := claudeRunner.Run(sess, promptPath, "", false)
	if err != nil {
		t.Fatalf("ClaudeCodeRunner.Run failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success to be true")
	}
	if calledName != "claude" {
		t.Errorf("expected command 'claude', got '%s'", calledName)
	}
	if len(calledArgs) < 4 || calledArgs[0] != "-p" || calledArgs[1] != "run agent prompt" || calledArgs[2] != "--model" || calledArgs[3] != "opus" {
		t.Errorf("expected args ['-p', 'run agent prompt', '--model', 'opus'], got %v", calledArgs)
	}

	codexRunner := &CodexRunner{WorkspaceDir: tmpDir}
	res, err = codexRunner.Run(sess, promptPath, "", false)
	if err != nil {
		t.Fatalf("CodexRunner.Run failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success to be true")
	}
	if calledName != "codex" {
		t.Errorf("expected command 'codex', got '%s'", calledName)
	}
	if len(calledArgs) < 2 || calledArgs[0] != "exec" || calledArgs[1] != "run agent prompt" {
		t.Errorf("expected args ['exec', 'run agent prompt'], got %v", calledArgs)
	}

	// RunSession must dispatch to the matching provider via sess.Agent.Provider.
	sess.Agent = session.AgentContract{Provider: "codex"}
	if err := session.SaveSession(tmpDir, sess); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}
	if err := RunSession(tmpDir, sessID, "", false, ""); err != nil {
		t.Fatalf("RunSession with codex provider failed: %v", err)
	}
	if calledName != "codex" {
		t.Errorf("expected RunSession to dispatch to codex, got '%s'", calledName)
	}

	if _, err := resolveRunner("unsupported-provider", tmpDir, ""); err == nil {
		t.Errorf("expected error for unsupported provider")
	}
}

func TestOpenCodeRunner_Run_UndefinedAgent(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "runner-test-opencode-err-*")
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

	dummyFile := filepath.Join(tmpDir, "dummy.txt")
	err = ioutil.WriteFile(dummyFile, []byte("dummy"), 0644)
	if err != nil {
		t.Fatalf("failed to write dummy: %v", err)
	}
	runGitCmd("add", "dummy.txt")
	runGitCmd("commit", "-m", "initial commit")

	sessID := "sess-opencode-err"
	sessDir := filepath.Join(tmpDir, ".kv", "sessions", sessID)
	err = os.MkdirAll(sessDir, 0755)
	if err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	promptPath := filepath.Join(sessDir, "opencode.md")
	err = ioutil.WriteFile(promptPath, []byte("run agent prompt"), 0644)
	if err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}

	sess := &session.Session{
		ID:        sessID,
		Goal:      "Test OpenCode execution validation",
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

	runner := &OpenCodeRunner{WorkspaceDir: tmpDir}
	_, err = runner.Run(sess, promptPath, "invalid_agent", false)
	if err == nil {
		t.Errorf("expected error for undefined agent, got nil")
	} else if !strings.Contains(err.Error(), "agent 'invalid_agent' is not defined") {
		t.Errorf("expected error to mention agent 'invalid_agent' not defined, got: %v", err)
	}
}

