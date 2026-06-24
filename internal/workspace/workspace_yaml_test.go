package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceYamlLoadSave(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "kv-ws-yaml-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := &WorkspaceYaml{
		Workspace: WorkspaceInfo{
			Name: "test-workspace",
			Apps: []App{
				{
					ID:    "app-1",
					Name:  "App One",
					Path:  "app1",
					Type:  "backend",
					Stack: "go",
				},
			},
		},
	}

	err = SaveWorkspaceYaml(tmpDir, ws)
	if err != nil {
		t.Fatalf("SaveWorkspaceYaml failed: %v", err)
	}

	loaded, err := LoadWorkspaceYaml(tmpDir)
	if err != nil {
		t.Fatalf("LoadWorkspaceYaml failed: %v", err)
	}

	if loaded.Workspace.Name != "test-workspace" {
		t.Errorf("expected workspace name 'test-workspace', got '%s'", loaded.Workspace.Name)
	}
	if len(loaded.Workspace.Apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(loaded.Workspace.Apps))
	} else {
		app := loaded.Workspace.Apps[0]
		if app.ID != "app-1" || app.Name != "App One" || app.Path != "app1" || app.Type != "backend" || app.Stack != "go" {
			t.Errorf("loaded app fields mismatch: %+v", app)
		}
	}
}

func TestFindWorkspaceYamlDir(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "kv-ws-yaml-find-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "src", "nested", "folders")
	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("failed to create subdirectories: %v", err)
	}

	// Verify not found before creation
	_, err = FindWorkspaceYamlDir(subDir)
	if err == nil {
		t.Error("expected error when searching for non-existent workspace config")
	}

	// Write empty config at tmpDir
	wsFile := filepath.Join(tmpDir, WorkspaceYamlFileName)
	err = ioutil.WriteFile(wsFile, []byte("workspace:\n  name: test"), 0644)
	if err != nil {
		t.Fatalf("failed to write workspace file: %v", err)
	}

	// Find should succeed walking up from subDir
	foundDir, err := FindWorkspaceYamlDir(subDir)
	if err != nil {
		t.Fatalf("FindWorkspaceYamlDir failed: %v", err)
	}
	
	realTmpDir, _ := filepath.EvalSymlinks(tmpDir)
	realFoundDir, _ := filepath.EvalSymlinks(foundDir)
	if realFoundDir != realTmpDir {
		t.Errorf("expected workspace dir '%s', got '%s'", realTmpDir, realFoundDir)
	}
}

func TestWorkspaceYamlValidation(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "kv-ws-val-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy app directory
	appDir := filepath.Join(tmpDir, "app-one")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("failed to create app dir: %v", err)
	}

	tests := []struct {
		name    string
		ws      *WorkspaceYaml
		wantErr bool
		msg     string
	}{
		{
			name: "valid workspace",
			ws: &WorkspaceYaml{
				Workspace: WorkspaceInfo{
					Name: "valid",
					Apps: []App{
						{
							ID:    "app-1",
							Name:  "App One",
							Path:  "app-one",
							Type:  "backend",
							Stack: "go",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty workspace name",
			ws: &WorkspaceYaml{
				Workspace: WorkspaceInfo{
					Name: "",
					Apps: []App{},
				},
			},
			wantErr: true,
			msg:     "workspace name cannot be empty",
		},
		{
			name: "missing app ID",
			ws: &WorkspaceYaml{
				Workspace: WorkspaceInfo{
					Name: "valid",
					Apps: []App{
						{
							Name:  "App One",
							Path:  "app-one",
							Type:  "backend",
							Stack: "go",
						},
					},
				},
			},
			wantErr: true,
			msg:     "missing required field 'id'",
		},
		{
			name: "duplicate app ID",
			ws: &WorkspaceYaml{
				Workspace: WorkspaceInfo{
					Name: "valid",
					Apps: []App{
						{
							ID:    "app-1",
							Name:  "App One",
							Path:  "app-one",
							Type:  "backend",
							Stack: "go",
						},
						{
							ID:    "app-1",
							Name:  "App Two",
							Path:  "app-one",
							Type:  "frontend",
							Stack: "ts",
						},
					},
				},
			},
			wantErr: true,
			msg:     "duplicate app ID: app-1",
		},
		{
			name: "missing required field name",
			ws: &WorkspaceYaml{
				Workspace: WorkspaceInfo{
					Name: "valid",
					Apps: []App{
						{
							ID:    "app-1",
							Path:  "app-one",
							Type:  "backend",
							Stack: "go",
						},
					},
				},
			},
			wantErr: true,
			msg:     "missing required field 'name'",
		},
		{
			name: "non-existent path",
			ws: &WorkspaceYaml{
				Workspace: WorkspaceInfo{
					Name: "valid",
					Apps: []App{
						{
							ID:    "app-1",
							Name:  "App One",
							Path:  "does-not-exist",
							Type:  "backend",
							Stack: "go",
						},
					},
				},
			},
			wantErr: true,
			msg:     "path 'does-not-exist' does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ws.Validate(tmpDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !containsString(err.Error(), tt.msg) {
				t.Errorf("Validate() error message '%s' expected to contain '%s'", err.Error(), tt.msg)
			}
		})
	}
}

func containsString(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || (len(substr) > 0 && (str[:len(substr)] == substr || str[len(str)-len(substr):] == substr || stringsContains(str, substr))))
}

func stringsContains(s, substr string) bool {
	// Simple lookup
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
