package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"strings"

	"kv/internal/boundary"
	"kv/internal/context"
	"kv/internal/diff"
	"kv/internal/opencode"
	"kv/internal/quality"
	"kv/internal/report"
	"kv/internal/session"
	"kv/internal/task"
)

// RunCommandOverride is a helper to override exec.Command in tests.
var RunCommandOverride = exec.Command

// AgentRunResult holds the details of agent execution.
type AgentRunResult struct {
	Success bool
	Output  string
	Error   error
}

// AgentRunner defines the interface for executing an AI agent on a session.
type AgentRunner interface {
	Run(sess *session.Session, promptPath string, agentName string, dryRun bool) (*AgentRunResult, error)
}

// OpenCodeRunner is the implementation of AgentRunner for OpenCode.
type OpenCodeRunner struct {
	WorkspaceDir string
}

// Run executes the OpenCode agent flow.
func (r *OpenCodeRunner) Run(sess *session.Session, promptPath string, agentName string, dryRun bool) (*AgentRunResult, error) {
	// 1. Boundary check before starting
	boundaryReport, err := boundary.ValidateSession(r.WorkspaceDir, sess, true)
	if err != nil {
		return nil, fmt.Errorf("pre-run boundary validation failed: %w", err)
	}
	if !boundaryReport.IsValid() {
		return nil, fmt.Errorf("cannot run session: staged/unstaged changes violate boundary constraints")
	}

	if dryRun {
		_ = session.LogEvent(r.WorkspaceDir, sess.ID, "dry_run", map[string]interface{}{
			"status": "passed",
		})

		fmt.Println("Dry run")
		fmt.Println()
		fmt.Printf("Session:\n- %s\n\n", sess.ID)
		fmt.Printf("Goal:\n- %s\n\n", sess.Goal)
		fmt.Println("Context files:")
		fmt.Printf("- .kv/sessions/%s/context.md\n", sess.ID)
		fmt.Printf("- .kv/sessions/%s/opencode.md\n\n", sess.ID)
		fmt.Println("Allowed read paths:")
		for _, p := range sess.Boundary.AllowedPaths {
			rel, _ := filepath.Rel(r.WorkspaceDir, p)
			fmt.Printf("- ./%s\n", rel)
		}
		fmt.Println()
		fmt.Println("Allowed write paths:")
		for _, p := range sess.Boundary.WritablePaths {
			rel, _ := filepath.Rel(r.WorkspaceDir, p)
			fmt.Printf("- ./%s\n", rel)
		}
		fmt.Println()
		fmt.Println("Quality commands:")
		for _, qc := range sess.Quality.Commands {
			fmt.Printf("- %s\n", qc)
		}
		fmt.Println()

		contextMDFile := filepath.Join(r.WorkspaceDir, ".kv", "sessions", sess.ID, "context.md")
		contextSizeStr := "0.0k tokens"
		if data, err := ioutil.ReadFile(contextMDFile); err == nil {
			tokens := len(data) / 4
			contextSizeStr = fmt.Sprintf("%.1fk tokens", float64(tokens)/1000.0)
		}
		fmt.Printf("Estimated context:\n- %s\n\n", contextSizeStr)
		fmt.Println("Boundary:\n- passed")

		return &AgentRunResult{
			Success: true,
			Output:  "Dry run completed successfully.",
		}, nil
	}

	// 2. Log opencode started
	_ = session.LogEvent(r.WorkspaceDir, sess.ID, "opencode_started", nil)

	// Verify opencode doctor
	healthy, doctorErr := opencode.Doctor()
	if doctorErr != nil || !healthy {
		fmt.Println("Warning: OpenCode configuration seems unhealthy. Run 'kv opencode doctor' or 'kv opencode install' to fix.")
	}

	// 3. Execution prompt notification
	fmt.Printf("\nExecuting OpenCode agent for session '%s'...\n", sess.ID)
	fmt.Printf("Agent prompt loaded from: %s\n", promptPath)
	fmt.Println("The agent will automatically perform changes inside your workspace allowed paths:")
	for _, p := range sess.Boundary.WritablePaths {
		fmt.Printf("  - %s\n", p)
	}

	// Read prompt file content
	promptBytes, err := ioutil.ReadFile(promptPath)
	if err != nil {
		_ = session.LogEvent(r.WorkspaceDir, sess.ID, "opencode_finished", map[string]interface{}{
			"status": "failed",
			"error":  fmt.Sprintf("failed to read prompt file: %v", err),
		})
		return nil, fmt.Errorf("failed to read prompt file: %w", err)
	}
	promptContent := string(promptBytes)

	// Validate agent name
	if agentName != "" {
		defined, err := opencode.IsAgentDefined(r.WorkspaceDir, agentName)
		if err != nil {
			return nil, fmt.Errorf("failed to validate agent '%s': %w", agentName, err)
		}
		if !defined {
			return nil, fmt.Errorf("agent '%s' is not defined in opencode.json. Please define it before running.", agentName)
		}
	}

	// Execute opencode run
	var args []string
	args = append(args, "run")
	if agentName != "" {
		args = append(args, "--agent", agentName)
	}
	args = append(args, promptContent)

	cmd := RunCommandOverride("opencode", args...)
	cmd.Dir = r.WorkspaceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	err = cmd.Run()
	if err != nil {
		_ = session.LogEvent(r.WorkspaceDir, sess.ID, "opencode_finished", map[string]interface{}{
			"status": "failed",
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("opencode execution failed: %w", err)
	}

	_ = session.LogEvent(r.WorkspaceDir, sess.ID, "opencode_finished", map[string]interface{}{
		"status": "success",
	})

	// 4. Quality Gates
	fmt.Println("\nRunning Quality Gates...")
	qualityResults, qualityPassed, err := quality.RunQualityGates(r.WorkspaceDir, sess)
	if err != nil {
		return nil, fmt.Errorf("quality gates execution failed: %w", err)
	}

	// 5. Diff Summarizer
	fmt.Println("\nGenerating Diff Summary...")
	_, err = diff.GenerateDiffSummary(r.WorkspaceDir, sess)
	if err != nil {
		fmt.Printf("Warning: failed to generate diff summary: %v\n", err)
	}

	// 6. Session Report
	fmt.Println("\nGenerating Session Report...")
	err = report.GenerateSessionReport(r.WorkspaceDir, sess, qualityResults, qualityPassed)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session report: %w", err)
	}

	fmt.Printf("\nSession run finished. Status: %s\n", sess.Status)
	return &AgentRunResult{
		Success: qualityPassed,
		Output:  "Agent run finished successfully.",
	}, nil
}

// RunSession executes the session flow.
func RunSession(workspaceDir, sessionID, agentName string, dryRun bool) error {
	// 1. Load session
	sess, err := session.LoadSession(workspaceDir, sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session '%s': %w", sessionID, err)
	}

	// 2. Validate session contract
	if err := boundary.ValidateSessionContract(workspaceDir, sess); err != nil {
		return fmt.Errorf("session validation failed: %w", err)
	}

	// 3. Make sure context is built
	promptPath := filepath.Join(workspaceDir, ".kv", "sessions", sessionID, "opencode.md")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		fmt.Printf("Session context prompt not found. Automatically building context for session %s...\n", sessionID)
		_, _, _, err = context.BuildSessionContext(workspaceDir, sessionID)
		if err != nil {
			return fmt.Errorf("failed to automatically build session context: %w", err)
		}
		if _, err := os.Stat(promptPath); os.IsNotExist(err) {
			return fmt.Errorf("session context prompt %s not found after auto-build. Please run 'kv context build --session %s' first", promptPath, sessionID)
		}
	}

	// 4. Resolve runner and execute
	var agentRunner AgentRunner
	if sess.Agent.Provider == "opencode" || sess.Agent.Provider == "" {
		agentRunner = &OpenCodeRunner{WorkspaceDir: workspaceDir}
	} else {
		return fmt.Errorf("unsupported agent provider: '%s'", sess.Agent.Provider)
	}

	if agentName == "" {
		agentName = sess.Agent.Name
	}

	result, err := agentRunner.Run(sess, promptPath, agentName, dryRun)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("agent execution succeeded, but quality gates or checks failed: %s", result.Output)
	}

	return nil
}

// RunTask executes a task using the specified runner. (Legacy compatible helper)
func RunTask(workspaceDir, workflowSlug, taskID, runnerType, agentName string) error {
	// 1. Build the context first to ensure .opencode/context.md is generated
	fmt.Printf("Building context for task %s in workflow %s...\n", taskID, workflowSlug)
	if err := context.BuildContext(workspaceDir, workflowSlug, taskID); err != nil {
		return fmt.Errorf("failed to build context pack before running task: %v", err)
	}

	// 2. Resolve runner type
	if runnerType == "" {
		runnerType = "opencode"
	}

	// If runnerType contains a slash (e.g. opencode/backend), split it
	if strings.Contains(runnerType, "/") {
		parts := strings.SplitN(runnerType, "/", 2)
		runnerType = parts[0]
		if agentName == "" {
			agentName = parts[1]
		}
	}

	// Load task to see if agent is defined in frontmatter
	workflowDir := filepath.Join(workspaceDir, ".kv", "workflows", workflowSlug)
	taskFile := filepath.Join(workflowDir, "tasks", taskID+".md")
	var parsedTask *task.Task
	if _, err := os.Stat(taskFile); err == nil {
		if t, err := task.ParseTask(taskFile); err == nil {
			parsedTask = t
		}
	}

	if agentName == "" && parsedTask != nil {
		if parsedTask.Frontmatter.Agent != "" {
			agentName = parsedTask.Frontmatter.Agent
		} else if strings.Contains(parsedTask.Frontmatter.AgentRunner, "/") {
			parts := strings.SplitN(parsedTask.Frontmatter.AgentRunner, "/", 2)
			agentName = parts[1]
		}
	}

	switch runnerType {
	case "opencode":
		fmt.Printf("Executing task using runner: %s\n", runnerType)
		if agentName != "" {
			fmt.Printf("Using agent: %s\n", agentName)
		}
		// Check if opencode is installed/healthy
		healthy, err := opencode.Doctor()
		if err != nil || !healthy {
			fmt.Println("Warning: OpenCode configuration seems unhealthy. Run 'kv opencode doctor' or 'kv opencode install' to fix.")
		}

		// Validate agent if specified
		if agentName != "" {
			defined, err := opencode.IsAgentDefined(workspaceDir, agentName)
			if err != nil {
				return fmt.Errorf("failed to validate agent '%s': %w", agentName, err)
			}
			if !defined {
				return fmt.Errorf("agent '%s' is not defined in opencode.json. Please define it before running.", agentName)
			}
		}

		promptContent := "Please read the task context file at `.opencode/context.md` and complete the task instructions described there."

		var args []string
		args = append(args, "run")
		if agentName != "" {
			args = append(args, "--agent", agentName)
		}
		args = append(args, promptContent)

		cmd := RunCommandOverride("opencode", args...)
		cmd.Dir = workspaceDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("opencode execution failed: %w", err)
		}

		fmt.Println("\nOpenCode Runner executed successfully.")
		return nil

	default:
		return fmt.Errorf("unsupported runner type: '%s'. Supported runners: opencode", runnerType)
	}
}
