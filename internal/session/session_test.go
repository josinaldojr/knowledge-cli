package session

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"kv/internal/workspace"
)

func TestSessionStartAndRoundtrip(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "kv-session-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dummy app path
	appDir := filepath.Join(tmpDir, "app-a")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("failed to create app dir: %v", err)
	}

	ws := &workspace.WorkspaceYaml{
		Workspace: workspace.WorkspaceInfo{
			Name: "test-workspace",
			Apps: []workspace.App{
				{
					ID:    "app1",
					Name:  "App 1",
					Path:  "app-a",
					Type:  "service",
					Stack: "go",
				},
			},
		},
	}

	goal := "Refactor API authentication"
	sess, err := StartSession(tmpDir, ws, goal, []string{"app1"}, "", "")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}

	if sess.Goal != goal {
		t.Errorf("expected goal '%s', got '%s'", goal, sess.Goal)
	}
	if len(sess.SelectedApps) != 1 || sess.SelectedApps[0] != "app1" {
		t.Errorf("expected selected apps ['app1'], got %v", sess.SelectedApps)
	}
	if sess.Status != "active" {
		t.Errorf("expected session status to be 'active', got '%s'", sess.Status)
	}

	// Verify allowed path is absolute and points to the app
	expectedPath := filepath.Clean(filepath.Join(tmpDir, "app-a"))
	realExpected, _ := filepath.EvalSymlinks(expectedPath)
	
	if len(sess.Boundary.AllowedPaths) != 1 {
		t.Fatalf("expected 1 allowed path, got %d", len(sess.Boundary.AllowedPaths))
	}
	realAllowed, _ := filepath.EvalSymlinks(sess.Boundary.AllowedPaths[0])
	if realAllowed != realExpected {
		t.Errorf("expected allowed path '%s', got '%s'", realExpected, realAllowed)
	}

	// Verify we can load it back
	loaded, err := LoadSession(tmpDir, sess.ID)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if loaded.ID != sess.ID {
		t.Errorf("loaded ID mismatch: expected '%s', got '%s'", sess.ID, loaded.ID)
	}
	if loaded.Goal != sess.Goal {
		t.Errorf("loaded goal mismatch: expected '%s', got '%s'", sess.Goal, loaded.Goal)
	}
	if len(loaded.Boundary.AllowedPaths) != 1 || loaded.Boundary.AllowedPaths[0] != sess.Boundary.AllowedPaths[0] {
		t.Errorf("loaded boundary paths mismatch: expected %v, got %v", sess.Boundary.AllowedPaths, loaded.Boundary.AllowedPaths)
	}
}

func TestSessionValidationErrors(t *testing.T) {
	ws := &workspace.WorkspaceYaml{
		Workspace: workspace.WorkspaceInfo{
			Name: "test-workspace",
			Apps: []workspace.App{
				{
					ID:    "app1",
					Name:  "App 1",
					Path:  "app-a",
					Type:  "service",
					Stack: "go",
				},
			},
		},
	}

	tmpDir, err := ioutil.TempDir("", "kv-session-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test empty goal
	_, err = StartSession(tmpDir, ws, "", []string{"app1"}, "", "")
	if err == nil {
		t.Error("expected error for empty session goal, got nil")
	}

	// Test empty apps
	_, err = StartSession(tmpDir, ws, "goal", []string{}, "", "")
	if err == nil {
		t.Error("expected error for empty apps selection, got nil")
	}

	// Test non-existent app selection
	_, err = StartSession(tmpDir, ws, "goal", []string{"nonexistent"}, "", "")
	if err == nil {
		t.Error("expected error for selecting unregistered app, got nil")
	}
}
