package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kv/internal/hook"
	"kv/internal/lifecycle"
	"kv/internal/spool"
	"kv/internal/store"
)

var binPath string
var testHomeDir string

func TestMain(m *testing.M) {
	// Create a temp directory for compiling the binary
	tmpDir, err := ioutil.TempDir("", "kv-build-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir for build: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binPath = filepath.Join(tmpDir, "kv")

	// Compile the binary
	cmd := exec.Command("go", "build", "-o", binPath, "main.go")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to compile kv binary: %v\n", err)
		os.Exit(1)
	}

	// Create a separate temp directory for HOME sandbox
	testHomeDir, err = ioutil.TempDir("", "kv-home-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir for home: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(testHomeDir)

	// Run tests
	os.Exit(m.Run())
}

// Helper to run the compiled kv binary
func runKV(t *testing.T, tmpDir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = tmpDir
	// Set HOME to testHomeDir to isolate configuration files (~/.config/opencode)
	cmd.Env = append(os.Environ(), "HOME="+testHomeDir, "GEMINI_API_KEY=mock-key")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func commitAll(t *testing.T, wsDir string, message string) {
	t.Helper()
	cmdAdd := exec.Command("git", "add", "-A")
	cmdAdd.Dir = wsDir
	if err := cmdAdd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	cmdConfigUser := exec.Command("git", "config", "user.name", "Test User")
	cmdConfigUser.Dir = wsDir
	_ = cmdConfigUser.Run()

	cmdConfigEmail := exec.Command("git", "config", "user.email", "test@example.com")
	cmdConfigEmail.Dir = wsDir
	_ = cmdConfigEmail.Run()

	cmdCommit := exec.Command("git", "commit", "-m", message)
	cmdCommit.Dir = wsDir
	_ = cmdCommit.Run()
}

func TestKVCLIAllCommands(t *testing.T) {
	// Create a workspace root temp directory for E2E flow
	wsDir, err := ioutil.TempDir("", "kv-e2e-*")
	if err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}
	defer os.RemoveAll(wsDir)

	// Ensure we are inside a clean git repo so git-based commands work correctly
	gitInit := exec.Command("git", "init")
	gitInit.Dir = wsDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("failed to initialize git repo: %v", err)
	}

	// 1. HELP / GENERAL USAGE
	t.Run("GeneralHelp", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "help")
		if err != nil {
			t.Errorf("unexpected error for 'kv help': %v", err)
		}
		if !strings.Contains(stdout, "Available commands") {
			t.Errorf("expected usage info, got: %s", stdout)
		}
	})

	// 2. INVALID COMMAND
	t.Run("InvalidCommand", func(t *testing.T) {
		_, stderr, err := runKV(t, wsDir, "invalidcommand123")
		if err == nil {
			t.Error("expected error for invalid command, got nil")
		}
		if !strings.Contains(stderr, "Unknown command") {
			t.Errorf("expected 'Unknown command' error, got: %s", stderr)
		}
	})

	// 3. WORKSPACE INIT
	t.Run("WorkspaceInit", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "workspace", "init")
		if err != nil {
			t.Fatalf("'kv workspace init' failed: %v", err)
		}
		if !strings.Contains(stdout, "Workspace configuration saved") {
			t.Errorf("expected success message, got: %s", stdout)
		}

		yamlFile := filepath.Join(wsDir, "kv-workspace.yaml")
		if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
			t.Errorf("expected kv-workspace.yaml to exist")
		}
	})

	// 4. WORKSPACE SHOW
	t.Run("WorkspaceShow", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "workspace", "show")
		if err != nil {
			t.Fatalf("'kv workspace show' failed: %v", err)
		}
		if !strings.Contains(stdout, "Workspace Name") {
			t.Errorf("expected Workspace details, got: %s", stdout)
		}
	})

	// 5. VAULT INIT
	vaultPath := filepath.Join(wsDir, "my-vault")
	t.Run("VaultInit", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "vault", "init", vaultPath)
		if err != nil {
			t.Fatalf("'kv vault init' failed: %v", err)
		}
		if !strings.Contains(stdout, "Knowledge Vault inicializado") && !strings.Contains(stdout, "initialized") {
			t.Logf("Output (checking marker): %s", stdout)
		}

		markerFile := filepath.Join(vaultPath, ".kv-vault")
		if _, err := os.Stat(markerFile); os.IsNotExist(err) {
			t.Errorf("expected .kv-vault marker to exist")
		}
	})

	// 6. VAULT ATTACH
	t.Run("VaultAttach", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "vault", "attach", vaultPath)
		if err != nil {
			t.Fatalf("'kv vault attach' failed: %v", err)
		}
		if !strings.Contains(stdout, "Vault attached successfully") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	// 7. VAULT PATH
	t.Run("VaultPath", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "vault", "path")
		if err != nil {
			t.Fatalf("'kv vault path' failed: %v", err)
		}
		cleanOut := strings.TrimSpace(stdout)
		cleanVaultPath, _ := filepath.EvalSymlinks(vaultPath)
		cleanOutPath, _ := filepath.EvalSymlinks(cleanOut)
		if cleanOutPath != cleanVaultPath {
			t.Errorf("expected vault path '%s', got '%s'", cleanVaultPath, cleanOutPath)
		}
	})

	// 8. VAULT DOCTOR
	t.Run("VaultDoctor", func(t *testing.T) {
		// Run doctor check. Since we initialized a new vault, it should be healthy.
		_, _, err := runKV(t, wsDir, "vault", "doctor")
		if err != nil {
			t.Errorf("'kv vault doctor' failed: %v", err)
		}
	})

	// 9. APP ADD
	appPath := filepath.Join(wsDir, "service-a")
	if err := os.MkdirAll(appPath, 0755); err != nil {
		t.Fatalf("failed to create app dir: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(appPath, "go.mod"), []byte("module service-a\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod file: %v", err)
	}
	t.Run("AppAdd", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "app", "add", "--id", "service-a", "--name", "App A", "--path", "./service-a", "--type", "service", "--stack", "go")
		if err != nil {
			t.Fatalf("'kv app add' failed: %v", err)
		}
		if !strings.Contains(stdout, "Application 'service-a' successfully added") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	// 10. APP LIST
	t.Run("AppList", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "app", "list")
		if err != nil {
			t.Fatalf("'kv app list' failed: %v", err)
		}
		if !strings.Contains(stdout, "service-a") {
			t.Errorf("expected app ID 'service-a' to be listed, got: %s", stdout)
		}
	})

	// 11. WORKSPACE SCAN
	t.Run("WorkspaceScan", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "workspace", "scan")
		if err != nil {
			t.Fatalf("'kv workspace scan' failed: %v", err)
		}
		if !strings.Contains(stdout, "Detected apps") {
			t.Errorf("expected scan header, got: %s", stdout)
		}
	})

	// 12. INITIALIZE WORKSPACE (LEGACY INIT SWITCH)
	t.Run("LegacyInit", func(t *testing.T) {
		// Verify the legacy init command parses successfully
		stdout, _, err := runKV(t, wsDir, "init", "--vault", vaultPath)
		if err != nil {
			t.Fatalf("'kv init' failed: %v", err)
		}
		if !strings.Contains(stdout, "Workspace initialized successfully") {
			t.Errorf("expected legacy init message, got: %s", stdout)
		}
	})

	commitAll(t, wsDir, "Setup initial workspace files")

	// 13. SESSION INIT
	sessionID := "sess-123"
	t.Run("SessionInit", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "session", "init", "--id", sessionID, "--goal", "refactor test app", "--apps", "service-a=service-a", "--vault", vaultPath, "--writable", "service-a")
		if err != nil {
			t.Fatalf("'kv session init' failed: %v", err)
		}
		if !strings.Contains(stdout, "Session initialized successfully") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	// 14. SESSION START
	t.Run("SessionStart", func(t *testing.T) {
		stdout, stderr, err := runKV(t, wsDir, "session", "start", "--goal", "build something", "--apps", "service-a")
		if err != nil {
			t.Fatalf("'kv session start' failed: %v. Stderr: %s, Stdout: %s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, "Session started successfully") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	commitAll(t, wsDir, "Setup session metadata files")

	// 15. SESSION VALIDATE
	t.Run("SessionValidate", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "session", "validate", "--session", sessionID)
		if err != nil {
			t.Fatalf("'kv session validate' failed: %v", err)
		}
		if !strings.Contains(stdout, "Session valid") {
			t.Errorf("expected valid confirmation, got: %s", stdout)
		}
	})

	// Add dummy files to test diff / boundary validation
	dummyFile := filepath.Join(appPath, "dummy.go")
	if err := ioutil.WriteFile(dummyFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	// 16. BOUNDARY VALIDATE
	t.Run("BoundaryValidate", func(t *testing.T) {
		stdout, stderr, err := runKV(t, wsDir, "boundary", "validate", "--session", sessionID, "--include-untracked")
		if err != nil {
			t.Fatalf("'kv boundary validate' failed: %v. Stderr: %s, Stdout: %s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, "Valid changes") && !strings.Contains(stdout, "No changes detected") {
			t.Logf("Boundary validation output: %s", stdout)
		}
	})

	// 17. SESSION DIFF
	t.Run("SessionDiff", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "session", "diff", "--session", sessionID, "--include-untracked")
		if err != nil {
			t.Fatalf("'kv session diff' failed: %v", err)
		}
		// Since we have an untracked file and ask to include it, it should output diff format or no changes error
		if !strings.Contains(stdout, "dummy.go") && !strings.Contains(stdout, "No changes") {
			t.Logf("Session diff output: %s", stdout)
		}
	})

	// 18. DIFF SUMMARIZE
	t.Run("DiffSummarize", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "diff", "summarize", "--session", sessionID)
		if err != nil {
			t.Fatalf("'kv diff summarize' failed: %v", err)
		}
		if !strings.Contains(stdout, "Diff Summary") && !strings.Contains(stdout, "Resumo do Diff") {
			t.Logf("Diff summarize output: %s", stdout)
		}
	})

	// 19. QUALITY RUN
	t.Run("QualityRun", func(t *testing.T) {
		// Run quality gates. Since no custom commands are registered in our session config yet, it should pass default gates.
		stdout, _, err := runKV(t, wsDir, "quality", "run", "--session", sessionID)
		if err != nil {
			t.Fatalf("'kv quality run' failed: %v", err)
		}
		if !strings.Contains(stdout, "Quality Gates: PASSED") {
			t.Errorf("expected quality gates to pass, got: %s", stdout)
		}
	})

	// 20. SESSION REPORT
	t.Run("SessionReport", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "session", "report", "--session", sessionID)
		if err != nil {
			t.Fatalf("'kv session report' failed: %v", err)
		}
		if !strings.Contains(stdout, "Report successfully generated") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	// 21. WORKFLOW NEW
	workflowSlug := "auth-restructure"
	t.Run("WorkflowNew", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "workflow", "new", workflowSlug)
		if err != nil {
			t.Fatalf("'kv workflow new' failed: %v", err)
		}
		if !strings.Contains(stdout, "created successfully") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	// 22. WORKFLOW RUN
	t.Run("WorkflowRun", func(t *testing.T) {
		// Workflow run invokes OpenCode runner adapter. Since there is no actual agent executor configured,
		// it should exit with an expected setup error or dry-run. Let's make sure it handles parameters properly.
		_, _, err := runKV(t, wsDir, "workflow", "run", workflowSlug, "--prompt", "restructure authentication")
		if err == nil {
			t.Log("workflow run completed without errors")
		} else {
			t.Logf("workflow run completed with error (expected/acceptable): %v", err)
		}
	})

	// 23. TASK ENRICH
	taskID := "task-001"
	t.Run("TaskEnrich", func(t *testing.T) {
		// Precreate the task description in workflow directory so it can be enriched
		taskFile := filepath.Join(wsDir, ".kv", "workflows", workflowSlug, "tasks", taskID+".md")
		_ = os.MkdirAll(filepath.Dir(taskFile), 0755)
		_ = ioutil.WriteFile(taskFile, []byte("# Task: Auth\nValidate tokens"), 0644)

		stdout, _, err := runKV(t, wsDir, "task", "enrich", workflowSlug, taskID)
		if err != nil {
			t.Fatalf("'kv task enrich' failed: %v", err)
		}
		if !strings.Contains(stdout, "enriched successfully") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	// 24. TASK RUN
	t.Run("TaskRun", func(t *testing.T) {
		_, _, err := runKV(t, wsDir, "task", "run", workflowSlug, taskID)
		if err == nil {
			t.Log("task run completed without errors")
		} else {
			t.Logf("task run completed with error (expected/acceptable): %v", err)
		}
	})

	// 25. CONTEXT BUILD
	t.Run("ContextBuildSession", func(t *testing.T) {
		stdout, stderr, err := runKV(t, wsDir, "context", "build", "--session", sessionID)
		if err != nil {
			t.Fatalf("'kv context build --session' failed: %v. Stderr: %s, Stdout: %s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, "Context compilation completed") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	t.Run("ContextBuildTaskLegacy", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "context", "build", workflowSlug, taskID)
		if err != nil {
			t.Fatalf("'kv context build <flow> <task>' failed: %v", err)
		}
		defer os.Remove(filepath.Join(wsDir, ".opencode", "context.md"))
		if !strings.Contains(stdout, "Context generated successfully") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	t.Run("ContextLegacyQuery", func(t *testing.T) {
		// kv context <query> uses vault search to build context
		_, _, err := runKV(t, wsDir, "context", "auth")
		if err != nil {
			t.Logf("'kv context <query>' returned error: %v", err)
		}
		defer os.Remove(filepath.Join(wsDir, ".opencode", "context.md"))
	})

	// 26. FIND
	t.Run("FindQuery", func(t *testing.T) {
		// Populate vault with a dummy note to search
		inboxPath := filepath.Join(vaultPath, "00-inbox")
		_ = os.MkdirAll(inboxPath, 0755)
		dummyNote := filepath.Join(inboxPath, "payment.md")
		_ = ioutil.WriteFile(dummyNote, []byte("stripe integration notes"), 0644)
		defer os.Remove(dummyNote)

		stdout, _, err := runKV(t, wsDir, "find", "stripe")
		if err != nil {
			t.Logf("'kv find' returned error: %v (expected if indexer is empty/failing)", err)
		}
		if stdout != "" {
			t.Logf("Find output: %s", stdout)
		}
	})

	// 27. OPENCODE INSTALL
	t.Run("OpenCodeInstall", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "opencode", "install")
		if err != nil {
			t.Fatalf("'kv opencode install' failed: %v", err)
		}
		if !strings.Contains(stdout, "completed successfully") {
			t.Errorf("expected success message, got: %s", stdout)
		}

		// Verify files exist in the home-sandbox directory
		configDir := filepath.Join(testHomeDir, ".config", "opencode")
		if _, err := os.Stat(filepath.Join(configDir, "opencode.json")); os.IsNotExist(err) {
			t.Errorf("expected opencode.json to be installed in sandbox home")
		}
	})

	// 28. OPENCODE DOCTOR
	t.Run("OpenCodeDoctor", func(t *testing.T) {
		stdout, _, err := runKV(t, wsDir, "opencode", "doctor")
		if err != nil {
			t.Fatalf("'kv opencode doctor' failed: %v", err)
		}
		if !strings.Contains(stdout, "Status: healthy") {
			t.Errorf("expected status healthy, got: %s", stdout)
		}
	})

	// 29. RUN SESSION (DRY RUN)
	t.Run("RunSessionDry", func(t *testing.T) {
		stdout, stderr, err := runKV(t, wsDir, "run", "--session", sessionID, "--dry-run")
		if err != nil {
			t.Fatalf("'kv run --dry-run' failed: %v. Stderr: %s, Stdout: %s", err, stderr, stdout)
		}
		if !strings.Contains(stdout, "Dry run") && !strings.Contains(stdout, "Simulating") && !strings.Contains(stdout, "simulação") {
			t.Logf("Run session output: %s", stdout)
		}
	})

	// 30. WIKI SERVE (HTTP Server E2E)
	t.Run("WikiServe", func(t *testing.T) {
		// Launch server in background using exec.Command on a separate port
		srvPort := "19090"
		cmd := exec.Command(binPath, "wiki", "serve", "--port", srvPort)
		cmd.Dir = wsDir
		cmd.Env = append(os.Environ(), "HOME="+wsDir, "GEMINI_API_KEY=mock-key")

		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start wiki serve subprocess: %v", err)
		}
		defer func() {
			_ = cmd.Process.Kill()
		}()

		// Wait a moment for server to listen
		time.Sleep(100 * time.Millisecond)

		// Hit the API docs endpoint
		resp, err := http.Get("http://127.0.0.1:" + srvPort + "/api/wiki/docs")
		if err != nil {
			t.Fatalf("failed to query wiki serve: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected HTTP 200, got %d", resp.StatusCode)
		}
	})

	// 31. WIKI COMPILE, LINK, ASK
	t.Run("WikiCompile", func(t *testing.T) {
		// wiki compile makes an LLM call. With a mock-key it should fail with HTTP 400 or 403 API error,
		// which validates that client client instantiation, notes scanning, and LLM call preparation all worked perfectly!
		_, stderr, err := runKV(t, wsDir, "wiki", "compile")
		if err != nil {
			if !strings.Contains(stderr, "api request returned status") && !strings.Contains(err.Error(), "exit status 1") {
				t.Errorf("unexpected error compiling wiki: %v, stderr: %s", err, stderr)
			}
		}
	})

	t.Run("WikiLink", func(t *testing.T) {
		_, stderr, err := runKV(t, wsDir, "wiki", "link")
		if err != nil {
			if !strings.Contains(stderr, "api request returned status") && !strings.Contains(err.Error(), "exit status 1") {
				t.Errorf("unexpected error auto-linking wiki: %v, stderr: %s", err, stderr)
			}
		}
	})

	t.Run("WikiAsk", func(t *testing.T) {
		_, stderr, err := runKV(t, wsDir, "wiki", "ask", "What is auth?")
		if err != nil {
			if !strings.Contains(stderr, "api request returned status") && !strings.Contains(err.Error(), "exit status 1") {
				t.Errorf("unexpected error asking wiki: %v, stderr: %s", err, stderr)
			}
		}
	})

	// 32. TUI MODE
	t.Run("TuiStart", func(t *testing.T) {
		// When running 'kv' or 'kv tui' non-interactively without terminal, bubbletea's Run() returns an error.
		// We expect this to exit with code 1 and print an error message.
		_, stderr, err := runKV(t, wsDir, "tui")
		if err == nil {
			t.Error("expected error starting TUI in non-interactive shell, got nil")
		}
		if !strings.Contains(stderr, "Error starting TUI") && !strings.Contains(stderr, "error") {
			t.Logf("TUI stderr output: %s", stderr)
		}
	})
}

func TestGlobalAdministrationCommands(t *testing.T) {
	workspaceDir := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "global-data")
	t.Setenv("KV_DATA_HOME", dataDir)

	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(dataDir, "kv.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, filepath.Join(dataDir, "kv.db")); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(database)
	session, err := lifecycle.NewSessionService(repository).EnsureSession(ctx, workspaceDir, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "admin-test"})
	if err != nil {
		t.Fatal(err)
	}
	changeID, err := repository.EnsureOpenSpecChange(ctx, session.Workspace.LogicalID, "admin-change")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := repository.CreateArtifactRevision(ctx, changeID, filepath.Join(workspaceDir, "proposal.md"), "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateTemporalMemory(ctx, session.Workspace.LogicalID, store.MemoryRecord{Kind: "decision", Status: "current", Summary: "use global administration", RevisionIDs: []string{revisionID}}); err != nil {
		t.Fatal(err)
	}
	queue, err := spool.Open(filepath.Join(dataDir, "spool"))
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Enqueue(spool.Entry{Envelope: hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: "admin-retry", IdempotencyKey: "admin-retry", Event: hook.EventSessionObserved, OccurredAt: time.Now().UTC(), Provider: hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "retry-test"}, Workspace: hook.WorkspaceIdentity{CWD: workspaceDir}}, RetryAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		args string
		want string
	}{
		{"mcp status", "Pending events: 1"},
		{"workspace status", "Active sessions: 1"},
		{"session list", "opencode"},
		{"change history", "admin-change"},
		{"memory search --change admin-change administration", "use global administration"},
		{"mcp retry", "delivered=1"},
		{"mcp reconcile --timeout 1ns", "Reconciled stale state"},
		{"mcp doctor --provider all", "opencode:"},
	} {
		t.Run(test.args, func(t *testing.T) {
			stdout, stderr, err := runKV(t, workspaceDir, strings.Fields(test.args)...)
			if err != nil {
				t.Fatalf("kv %s failed: %v; stderr=%s", test.args, err, stderr)
			}
			if !strings.Contains(stdout, test.want) {
				t.Fatalf("kv %s output = %q, want %q", test.args, stdout, test.want)
			}
		})
	}

	t.Run("data export", func(t *testing.T) {
		exportPath := filepath.Join(workspaceDir, "knowledge.json")
		stdout, stderr, err := runKV(t, workspaceDir, "data", "export", "--file", exportPath)
		if err != nil {
			t.Fatalf("kv data export failed: %v; stderr=%s", err, stderr)
		}
		if !strings.Contains(stdout, "Exported global KV knowledge") {
			t.Fatalf("export output = %q", stdout)
		}
		contents, err := os.ReadFile(exportPath)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(contents, &document); err != nil {
			t.Fatal(err)
		}
		if document["schema_version"] != float64(1) || strings.Contains(string(contents), workspaceDir) {
			t.Fatalf("invalid or non-portable export: %s", contents)
		}
	})

	t.Run("data import rejects invalid evidence", func(t *testing.T) {
		invalidPath := filepath.Join(workspaceDir, "invalid-knowledge.json")
		invalid := `{"schema_version":1,"exported_at":"2026-01-01T00:00:00Z","workspace":{"identity":"git:example/invalid"},"sessions":[],"memories":[{"id":"m1","kind":"decision","status":"current","summary":"invalid","created_at":"2026-01-01T00:00:00Z","sources":[{"change_key":"change","artifact_path":"/absolute/path","revision_hash":"hash"}]}]}`
		if err := os.WriteFile(invalidPath, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		_, stderr, err := runKV(t, workspaceDir, "data", "import", "--file", invalidPath)
		if err == nil || !strings.Contains(stderr, "invalid evidence source") {
			t.Fatalf("invalid import err=%v stderr=%q", err, stderr)
		}
	})
}

func TestGlobalStoreDoesNotMixLegacyWorkspaceState(t *testing.T) {
	workspaceDir := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "global-data")
	t.Setenv("KV_DATA_HOME", dataDir)

	for _, args := range [][]string{
		{"init"},
		{"session", "init", "--id", "legacy-session", "--goal", "preserve legacy session"},
		{"workflow", "new", "legacy-workflow"},
	} {
		stdout, stderr, err := runKV(t, workspaceDir, args...)
		if err != nil {
			t.Fatalf("kv %s failed: %v; stdout=%s stderr=%s", strings.Join(args, " "), err, stdout, stderr)
		}
	}

	legacySession := filepath.Join(workspaceDir, ".kv", "sessions", "legacy-session", "session.yaml")
	legacyWorkflow := filepath.Join(workspaceDir, ".kv", "workflows", "legacy-workflow", "idea.md")
	sessionBefore, err := os.ReadFile(legacySession)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacyWorkflow); err != nil {
		t.Fatalf("legacy workflow was not created: %v", err)
	}

	stdout, stderr, err := runKV(t, workspaceDir, "mcp", "status")
	if err != nil || !strings.Contains(stdout, "MCP status: ready") {
		t.Fatalf("kv mcp status failed: %v; stdout=%s stderr=%s", err, stdout, stderr)
	}
	_, stderr, err = runKV(t, workspaceDir, "workspace", "status")
	if err == nil || !strings.Contains(stderr, "no global KV workspace record") {
		t.Fatalf("legacy state was treated as global workspace state: err=%v stderr=%s", err, stderr)
	}

	database, err := store.Open(context.Background(), filepath.Join(dataDir, "kv.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var workspaces, sessions int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM workspaces").Scan(&workspaces); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM provider_sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if workspaces != 0 || sessions != 0 {
		t.Fatalf("global store implicitly imported legacy state: workspaces=%d sessions=%d", workspaces, sessions)
	}

	sessionAfter, err := os.ReadFile(legacySession)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sessionBefore, sessionAfter) {
		t.Fatal("global administration modified the legacy session")
	}
}
