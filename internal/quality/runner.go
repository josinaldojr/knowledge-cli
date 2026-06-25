package quality

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"kv/internal/policy"
	"kv/internal/session"
)

// RunQualityGates executes all quality commands configured in the session.
// It runs them using the system shell, validates them against the Policy Engine,
// captures output, logs events, and returns the detailed result logs and a boolean indicating if all passed.
func RunQualityGates(workspaceDir string, sess *session.Session) (map[string]string, bool, error) {
	results := make(map[string]string)
	allPassed := true

	_ = session.LogEvent(workspaceDir, sess.ID, "quality_started", nil)

	for _, cmdLine := range sess.Quality.Commands {
		cmdClean := strings.TrimSpace(cmdLine)
		if cmdClean == "" {
			continue
		}

		// 1. Validate with Policy Engine
		allowed, err := policy.ValidateCommand(cmdClean, sess.Policy)
		if !allowed || err != nil {
			errStr := fmt.Sprintf("Blocked by Policy: %v", err)
			results[cmdClean] = errStr
			allPassed = false
			_ = session.LogEvent(workspaceDir, sess.ID, "quality_finished", map[string]interface{}{
				"command": cmdClean,
				"status":  "failed",
				"details": errStr,
			})
			continue
		}

		// 2. Execute command
		// Run from the first app's resolved directory or workspace directory
		var runDir string
		if len(sess.Apps) > 0 {
			runDir = sess.Apps[0].Path
			if !filepath.IsAbs(runDir) {
				runDir = filepath.Clean(filepath.Join(workspaceDir, runDir))
			}
		} else {
			runDir = workspaceDir
		}

		// For shell execution on unix-like OS
		cmd := exec.Command("sh", "-c", cmdClean)
		cmd.Dir = runDir

		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		runErr := cmd.Run()

		output := stdoutBuf.String()
		stderrStr := stderrBuf.String()
		combined := output
		if stderrStr != "" {
			if combined != "" {
				combined += "\n"
			}
			combined += "Stderr:\n" + stderrStr
		}

		if runErr != nil {
			allPassed = false
			results[cmdClean] = fmt.Sprintf("FAILED\n%s", combined)
			_ = session.LogEvent(workspaceDir, sess.ID, "quality_finished", map[string]interface{}{
				"command": cmdClean,
				"status":  "failed",
			})
		} else {
			results[cmdClean] = "PASSED"
			_ = session.LogEvent(workspaceDir, sess.ID, "quality_finished", map[string]interface{}{
				"command": cmdClean,
				"status":  "passed",
			})
		}
	}

	return results, allPassed, nil
}
