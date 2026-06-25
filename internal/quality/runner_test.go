package quality

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kv/internal/session"
)

func TestRunQualityGates(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "quality-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sess := &session.Session{
		ID:   "sess-quality-test",
		Goal: "test quality gates",
		Boundary: session.Boundary{
			AllowedPaths:  []string{tmpDir},
			WritablePaths: []string{tmpDir},
		},
		Quality: session.QualityContract{
			Enabled: true,
			Commands: []string{
				"echo hello",
				"false",
			},
		},
		Policy: session.PolicyContract{
			AllowEnvRead:           true,
			AllowDependencyInstall: "true",
			AllowMigrations:        "true",
			AllowDocker:            true,
		},
	}

	results, passed, err := RunQualityGates(tmpDir, sess)
	if err != nil {
		t.Fatalf("RunQualityGates failed: %v", err)
	}

	if passed {
		t.Error("expected quality gates to fail since one command is 'false'")
	}

	resEcho, ok := results["echo hello"]
	if !ok || resEcho != "PASSED" {
		t.Errorf("expected 'echo hello' to be PASSED, got: %s", resEcho)
	}

	resFalse, ok := results["false"]
	if !ok || !strings.Contains(resFalse, "FAILED") {
		t.Errorf("expected 'false' to be FAILED, got: %s", resFalse)
	}
}

func TestRunQualityGatesGoTestNoFiles(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "quality-test-go-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy go.mod file
	goModContent := "module dummy\n\ngo 1.20\n"
	err = ioutil.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	sess := &session.Session{
		ID:   "sess-quality-go-test",
		Goal: "test quality go test",
		Boundary: session.Boundary{
			AllowedPaths:  []string{tmpDir},
			WritablePaths: []string{tmpDir},
		},
		Quality: session.QualityContract{
			Enabled: true,
			Commands: []string{
				"go test ./...",
			},
		},
		Policy: session.PolicyContract{
			AllowDependencyInstall: "true",
			AllowMigrations:        "true",
			AllowDocker:            true,
		},
	}

	results, passed, err := RunQualityGates(tmpDir, sess)
	if err != nil {
		t.Fatalf("RunQualityGates failed: %v", err)
	}

	if passed {
		t.Error("expected quality gates to fail since there are no Go files")
	}

	resGoTest, ok := results["go test ./..."]
	if !ok || !strings.Contains(resGoTest, "Tip: Go tests require at least one Go file") {
		t.Errorf("expected suggestion tip in results, got: %s", resGoTest)
	}
}

