package context

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kv/internal/session"
	"kv/internal/workspace"
)

func TestBuildContext(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "context-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	workflowSlug := "test-flow"
	taskID := "001-setup"

	workflowDir := filepath.Join(tmpDir, ".kv", "workflows", workflowSlug)
	contextDir := filepath.Join(workflowDir, "context")
	err = os.MkdirAll(contextDir, 0755)
	if err != nil {
		t.Fatalf("failed to create context dir: %v", err)
	}

	contextPackPath := filepath.Join(contextDir, taskID+".context.md")
	expectedContent := "my context pack content"
	err = ioutil.WriteFile(contextPackPath, []byte(expectedContent), 0644)
	if err != nil {
		t.Fatalf("failed to write mock context pack: %v", err)
	}

	err = BuildContext(tmpDir, workflowSlug, taskID)
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	opencodeFile := filepath.Join(tmpDir, ".opencode", "context.md")
	if _, err := os.Stat(opencodeFile); os.IsNotExist(err) {
		t.Fatalf(".opencode/context.md not created")
	}

	data, err := ioutil.ReadFile(opencodeFile)
	if err != nil {
		t.Fatalf("failed to read .opencode/context.md: %v", err)
	}

	if string(data) != expectedContent {
		t.Errorf("expected '%s', got '%s'", expectedContent, string(data))
	}
}

func TestBuildSessionContext(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "session-context-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Create mock workspace config
	ws := &workspace.WorkspaceYaml{
		Workspace: workspace.WorkspaceInfo{
			Name: "my-test-workspace",
			Apps: []workspace.App{
				{
					ID:    "app1",
					Name:  "Application One",
					Path:  "src/app1",
					Type:  "service",
					Stack: "go",
				},
			},
		},
	}
	err = workspace.SaveWorkspaceYaml(tmpDir, ws)
	if err != nil {
		t.Fatalf("failed to save mock workspace yaml: %v", err)
	}

	// 2. Create physical app directories and files (including ignored ones)
	appDir := filepath.Join(tmpDir, "src", "app1")
	err = os.MkdirAll(appDir, 0755)
	if err != nil {
		t.Fatalf("failed to create app dir: %v", err)
	}

	// Create subdirs
	ignoredSubdir := filepath.Join(appDir, "node_modules")
	_ = os.MkdirAll(ignoredSubdir, 0755)
	nestedDir := filepath.Join(appDir, "auth")
	_ = os.MkdirAll(nestedDir, 0755)

	// Create files
	_ = ioutil.WriteFile(filepath.Join(appDir, "main.go"), []byte("package main"), 0644)
	_ = ioutil.WriteFile(filepath.Join(nestedDir, "auth.go"), []byte("package auth"), 0644)
	_ = ioutil.WriteFile(filepath.Join(ignoredSubdir, "ignored.js"), []byte("console.log()"), 0644)

	// 3. Create mock session
	sessionID := "sess-test-12345"
	sessionDir := filepath.Join(tmpDir, ".kv", "sessions", sessionID)
	err = os.MkdirAll(sessionDir, 0755)
	if err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	sess := &session.Session{
		ID:           sessionID,
		Goal:         "Refactor authentication logic",
		SelectedApps: []string{"app1"},
		CreatedAt:    time.Now(),
		Boundary: session.Boundary{
			AllowedPaths: []string{filepath.Join(tmpDir, "src", "app1")},
		},
		Status:       "active",
	}

	// Save session
	err = session.SaveSession(tmpDir, sess)
	if err != nil {
		t.Fatalf("failed to write mock session: %v", err)
	}

	// 4. Create mock active vault
	vaultDir := filepath.Join(tmpDir, "knowledge-vault")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = ioutil.WriteFile(filepath.Join(vaultDir, ".kv-vault"), []byte(""), 0644)
	// Active vault link
	_ = workspace.SaveConfig(tmpDir, &workspace.Config{VaultPath: "knowledge-vault"})

	// App-specific vault document
	appDoc := filepath.Join(vaultDir, "app1.md")
	vaultContent := "# Vault App1 Doc\nThis document describes App1 auth."
	_ = ioutil.WriteFile(appDoc, []byte(vaultContent), 0644)

	// 5. Execute BuildSessionContext
	files, processed, warnings, err := BuildSessionContext(tmpDir, sessionID)
	if err != nil {
		t.Fatalf("BuildSessionContext failed: %v", err)
	}
	_ = warnings

	if processed != 1 {
		t.Errorf("expected 1 processed app, got %d", processed)
	}
	if len(files) != 5 {
		t.Errorf("expected 5 generated files, got %d", len(files))
	}

	// Assert manifest exists and is valid
	manifestFile := filepath.Join(tmpDir, ".kv", "sessions", sessionID, "context", "context-manifest.json")
	if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
		t.Error("context-manifest.json was not created")
	}
	mBytes, _ := ioutil.ReadFile(manifestFile)
	var manifest Manifest
	_ = json.Unmarshal(mBytes, &manifest)
	if manifest.SessionID != sessionID {
		t.Errorf("manifest session ID mismatch: expected '%s', got '%s'", sessionID, manifest.SessionID)
	}
	if len(manifest.Apps) != 1 || manifest.Apps[0].AppID != "app1" {
		t.Errorf("manifest apps mismatch: %+v", manifest.Apps)
	}

	// Assert global context contents
	globalFile := filepath.Join(tmpDir, ".kv", "sessions", sessionID, "context", "global.context.md")
	gBytes, _ := ioutil.ReadFile(globalFile)
	gStr := string(gBytes)
	if !strings.Contains(gStr, "Refactor authentication logic") {
		t.Error("global context missing session goal")
	}
	if !strings.Contains(gStr, "Allowed Paths") {
		t.Error("global context missing allowed paths section")
	}

	// Assert app context contents
	appContextFile := filepath.Join(tmpDir, ".kv", "sessions", sessionID, "context", "apps", "app1.context.md")
	aBytes, _ := ioutil.ReadFile(appContextFile)
	aStr := string(aBytes)
	if !strings.Contains(aStr, "Application One") {
		t.Error("app context missing app name")
	}
	if !strings.Contains(aStr, "main.go") || !strings.Contains(aStr, "auth.go") {
		t.Error("app context missing file tree elements")
	}
	// Verify ignored folder is skipped in file tree
	if strings.Contains(aStr, "ignored.js") || strings.Contains(aStr, "node_modules") {
		t.Error("app context file tree should not contain ignored folders/files")
	}
	// Verify associated vault file contents are integrated
	if !strings.Contains(aStr, "Vault App1 Doc") {
		t.Error("app context should contain vault document content summary")
	}
	// Verify candidate files list has auth.go (matching keyword "authentication" from goal)
	if !strings.Contains(aStr, "auth/auth.go") {
		t.Error("app context candidates should match keywords from the goal and suggest auth/auth.go")
	}

	// 6. Test Error Cases
	// Test nonexistent session
	_, _, _, err = BuildSessionContext(tmpDir, "sess-nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session ID, got nil")
	}

	// Test app nonexistent in workspace (mismatch)
	sess.SelectedApps = []string{"app-mismatch"}
	_ = session.SaveSession(tmpDir, sess)

	_, _, _, err = BuildSessionContext(tmpDir, sessionID)
	if err == nil {
		t.Error("expected error when app in session is not registered in workspace, got nil")
	}
}
