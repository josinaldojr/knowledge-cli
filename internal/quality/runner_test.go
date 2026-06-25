package quality

import (
	"io/ioutil"
	"os"
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
