package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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

// cliAgentSpec describes how to invoke a provider's non-interactive CLI. Every
// supported provider (OpenCode, Claude Code, Codex) implements AgentRunner by
// filling in this spec and delegating to runCLIAgent, so the boundary check,
// dry-run summary, quality gates, diff summary, and session report stay
// identical across providers.
type cliAgentSpec struct {
	// Label is the human-readable provider name used in output messages.
	Label string
	// Binary is the executable invoked on PATH (e.g. "opencode", "claude", "codex").
	Binary string
	// Doctor reports whether the provider CLI looks usable. A false/degraded
	// result only produces a warning; it never blocks execution.
	Doctor func() (bool, error)
	// ValidateAgent optionally checks that a named sub-agent exists before
	// running. Providers without an agent registry leave this nil.
	ValidateAgent func(workspaceDir, agentName string) (bool, error)
	// BuildArgs builds the CLI arguments for a non-interactive run.
	BuildArgs func(promptContent, agentName, model string) []string
}

// runCLIAgent implements the shared AgentRunner flow for any provider CLI
// that can be driven non-interactively with a prompt string.
func runCLIAgent(spec cliAgentSpec, workspaceDir, model string, sess *session.Session, promptPath, agentName string, dryRun bool) (*AgentRunResult, error) {
	// 1. Boundary check before starting
	boundaryReport, err := boundary.ValidateSession(workspaceDir, sess, true)
	if err != nil {
		return nil, fmt.Errorf("pre-run boundary validation failed: %w", err)
	}
	if !boundaryReport.IsValid() {
		return nil, fmt.Errorf("cannot run session: staged/unstaged changes violate boundary constraints")
	}

	if dryRun {
		_ = session.LogEvent(workspaceDir, sess.ID, "dry_run", map[string]interface{}{
			"status": "passed",
		})

		fmt.Println("Dry run")
		fmt.Println()
		fmt.Printf("Session:\n- %s\n\n", sess.ID)
		fmt.Printf("Goal:\n- %s\n\n", sess.Goal)
		fmt.Printf("Runner:\n- %s\n\n", spec.Label)
		fmt.Println("Context files:")
		fmt.Printf("- .kv/sessions/%s/context.md\n", sess.ID)
		fmt.Printf("- .kv/sessions/%s/opencode.md\n\n", sess.ID)
		fmt.Println("Allowed read paths:")
		for _, p := range sess.Boundary.AllowedPaths {
			rel, _ := filepath.Rel(workspaceDir, p)
			fmt.Printf("- ./%s\n", rel)
		}
		fmt.Println()
		fmt.Println("Allowed write paths:")
		for _, p := range sess.Boundary.WritablePaths {
			rel, _ := filepath.Rel(workspaceDir, p)
			fmt.Printf("- ./%s\n", rel)
		}
		fmt.Println()
		fmt.Println("Quality commands:")
		for _, qc := range sess.Quality.Commands {
			fmt.Printf("- %s\n", qc)
		}
		fmt.Println()

		contextMDFile := filepath.Join(workspaceDir, ".kv", "sessions", sess.ID, "context.md")
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

	// 2. Log agent started
	_ = session.LogEvent(workspaceDir, sess.ID, spec.Binary+"_started", nil)

	// Verify the provider CLI looks usable. A failed/unavailable check never
	// blocks the run; it only surfaces a warning, matching the tolerant
	// degradation behavior documented for provider adapters.
	if spec.Doctor != nil {
		healthy, doctorErr := spec.Doctor()
		if doctorErr != nil || !healthy {
			fmt.Printf("Warning: %s CLI ('%s') was not found or is not configured. Install it and ensure it's on PATH before running.\n", spec.Label, spec.Binary)
		}
	}

	// 3. Execution prompt notification
	fmt.Printf("\nExecuting %s agent for session '%s'...\n", spec.Label, sess.ID)
	fmt.Printf("Agent prompt loaded from: %s\n", promptPath)
	fmt.Println("The agent will automatically perform changes inside your workspace allowed paths:")
	for _, p := range sess.Boundary.WritablePaths {
		fmt.Printf("  - %s\n", p)
	}

	// Read prompt file content
	promptBytes, err := ioutil.ReadFile(promptPath)
	if err != nil {
		_ = session.LogEvent(workspaceDir, sess.ID, spec.Binary+"_finished", map[string]interface{}{
			"status": "failed",
			"error":  fmt.Sprintf("failed to read prompt file: %v", err),
		})
		return nil, fmt.Errorf("failed to read prompt file: %w", err)
	}
	promptContent := string(promptBytes)

	// Validate agent name, when the provider supports named sub-agents.
	if agentName != "" && spec.ValidateAgent != nil {
		defined, err := spec.ValidateAgent(workspaceDir, agentName)
		if err != nil {
			return nil, fmt.Errorf("failed to validate agent '%s': %w", agentName, err)
		}
		if !defined {
			return nil, fmt.Errorf("agent '%s' is not defined in opencode.json. Please define it before running.", agentName)
		}
	}

	args := spec.BuildArgs(promptContent, agentName, model)
	if err := execProviderCLI(workspaceDir, spec.Binary, args); err != nil {
		_ = session.LogEvent(workspaceDir, sess.ID, spec.Binary+"_finished", map[string]interface{}{
			"status": "failed",
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("%s execution failed: %w", spec.Label, err)
	}

	_ = session.LogEvent(workspaceDir, sess.ID, spec.Binary+"_finished", map[string]interface{}{
		"status": "success",
	})

	// 4. Quality Gates
	fmt.Println("\nRunning Quality Gates...")
	qualityResults, qualityPassed, err := quality.RunQualityGates(workspaceDir, sess)
	if err != nil {
		return nil, fmt.Errorf("quality gates execution failed: %w", err)
	}

	// 5. Diff Summarizer
	fmt.Println("\nGenerating Diff Summary...")
	_, err = diff.GenerateDiffSummary(workspaceDir, sess)
	if err != nil {
		fmt.Printf("Warning: failed to generate diff summary: %v\n", err)
	}

	// 6. Session Report
	fmt.Println("\nGenerating Session Report...")
	err = report.GenerateSessionReport(workspaceDir, sess, qualityResults, qualityPassed)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session report: %w", err)
	}

	fmt.Printf("\nSession run finished. Status: %s\n", sess.Status)
	return &AgentRunResult{
		Success: qualityPassed,
		Output:  "Agent run finished successfully.",
	}, nil
}

// execProviderCLI runs a provider binary inside workspaceDir, streaming its
// stdio and printing a periodic progress heartbeat.
func execProviderCLI(workspaceDir, binary string, args []string) error {
	cmd := RunCommandOverride(binary, args...)
	cmd.Dir = workspaceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return RunCommandWithProgress(cmd)
}

// binaryOnPath reports whether name is resolvable on PATH. It is the doctor
// check for providers that need nothing beyond the executable itself.
func binaryOnPath(name string) (bool, error) {
	_, err := exec.LookPath(name)
	return err == nil, nil
}

func opencodeArgs(promptContent, agentName, model string) []string {
	args := []string{"run"}
	if agentName != "" {
		args = append(args, "--agent", agentName)
	}
	if model != "" {
		args = append(args, "-m", model)
	}
	args = append(args, promptContent)
	return args
}

func claudeCodeArgs(promptContent, _, model string) []string {
	// "-p" runs Claude Code non-interactively ("print mode"): it executes the
	// prompt to completion and exits instead of opening an interactive session.
	args := []string{"-p", promptContent}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

func codexArgs(promptContent, _, model string) []string {
	// "exec" is the Codex CLI's non-interactive automation entry point.
	args := []string{"exec", promptContent}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

// OpenCodeRunner is the AgentRunner implementation for OpenCode.
type OpenCodeRunner struct {
	WorkspaceDir string
	Model        string
}

func (r *OpenCodeRunner) Run(sess *session.Session, promptPath string, agentName string, dryRun bool) (*AgentRunResult, error) {
	spec := cliAgentSpec{
		Label:         "OpenCode",
		Binary:        "opencode",
		Doctor:        opencode.Doctor,
		ValidateAgent: opencode.IsAgentDefined,
		BuildArgs:     opencodeArgs,
	}
	return runCLIAgent(spec, r.WorkspaceDir, r.Model, sess, promptPath, agentName, dryRun)
}

// ClaudeCodeRunner is the AgentRunner implementation for Claude Code, driven
// through the `claude` CLI in non-interactive print mode.
type ClaudeCodeRunner struct {
	WorkspaceDir string
	Model        string
}

func (r *ClaudeCodeRunner) Run(sess *session.Session, promptPath string, agentName string, dryRun bool) (*AgentRunResult, error) {
	spec := cliAgentSpec{
		Label:     "Claude Code",
		Binary:    "claude",
		Doctor:    func() (bool, error) { return binaryOnPath("claude") },
		BuildArgs: claudeCodeArgs,
	}
	return runCLIAgent(spec, r.WorkspaceDir, r.Model, sess, promptPath, agentName, dryRun)
}

// CodexRunner is the AgentRunner implementation for Codex, driven through the
// `codex exec` non-interactive CLI entry point.
type CodexRunner struct {
	WorkspaceDir string
	Model        string
}

func (r *CodexRunner) Run(sess *session.Session, promptPath string, agentName string, dryRun bool) (*AgentRunResult, error) {
	spec := cliAgentSpec{
		Label:     "Codex",
		Binary:    "codex",
		Doctor:    func() (bool, error) { return binaryOnPath("codex") },
		BuildArgs: codexArgs,
	}
	return runCLIAgent(spec, r.WorkspaceDir, r.Model, sess, promptPath, agentName, dryRun)
}

// resolveRunner maps a provider identifier to its AgentRunner. Both
// "claude-code" (the canonical name used elsewhere in kv, e.g. `kv mcp doctor
// --provider claude-code`) and the bare "claude" binary name are accepted.
func resolveRunner(provider, workspaceDir, model string) (AgentRunner, error) {
	switch provider {
	case "opencode", "":
		return &OpenCodeRunner{WorkspaceDir: workspaceDir, Model: model}, nil
	case "claude-code", "claude":
		return &ClaudeCodeRunner{WorkspaceDir: workspaceDir, Model: model}, nil
	case "codex":
		return &CodexRunner{WorkspaceDir: workspaceDir, Model: model}, nil
	default:
		return nil, fmt.Errorf("unsupported agent provider: '%s'. Supported providers: opencode, claude-code, codex", provider)
	}
}

// RunSession executes the session flow.
func RunSession(workspaceDir, sessionID, agentName string, dryRun bool, model string) error {
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
	agentRunner, err := resolveRunner(sess.Agent.Provider, workspaceDir, model)
	if err != nil {
		return err
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
func RunTask(workspaceDir, workflowSlug, taskID, runnerType, agentName string, model string) error {
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

	var spec cliAgentSpec
	switch runnerType {
	case "opencode":
		spec = cliAgentSpec{Label: "OpenCode", Binary: "opencode", Doctor: opencode.Doctor, ValidateAgent: opencode.IsAgentDefined, BuildArgs: opencodeArgs}
	case "claude-code", "claude":
		spec = cliAgentSpec{Label: "Claude Code", Binary: "claude", Doctor: func() (bool, error) { return binaryOnPath("claude") }, BuildArgs: claudeCodeArgs}
	case "codex":
		spec = cliAgentSpec{Label: "Codex", Binary: "codex", Doctor: func() (bool, error) { return binaryOnPath("codex") }, BuildArgs: codexArgs}
	default:
		return fmt.Errorf("unsupported runner type: '%s'. Supported runners: opencode, claude-code, codex", runnerType)
	}

	fmt.Printf("Executing task using runner: %s\n", runnerType)
	if agentName != "" {
		fmt.Printf("Using agent: %s\n", agentName)
	}

	if spec.Doctor != nil {
		healthy, err := spec.Doctor()
		if err != nil || !healthy {
			fmt.Printf("Warning: %s CLI ('%s') was not found or is not configured. Install it and ensure it's on PATH before running.\n", spec.Label, spec.Binary)
		}
	}

	if agentName != "" && spec.ValidateAgent != nil {
		defined, err := spec.ValidateAgent(workspaceDir, agentName)
		if err != nil {
			return fmt.Errorf("failed to validate agent '%s': %w", agentName, err)
		}
		if !defined {
			return fmt.Errorf("agent '%s' is not defined in opencode.json. Please define it before running.", agentName)
		}
	}

	promptContent := "Please read the task context file at `.opencode/context.md` and complete the task instructions described there."
	args := spec.BuildArgs(promptContent, agentName, model)
	if err := execProviderCLI(workspaceDir, spec.Binary, args); err != nil {
		return fmt.Errorf("%s execution failed: %w", spec.Label, err)
	}

	fmt.Printf("\n%s Runner executed successfully.\n", spec.Label)
	return nil
}

// RunCommandWithProgress executes a command and prints a periodic progress heartbeat.
func RunCommandWithProgress(cmd *exec.Cmd) error {
	done := make(chan struct{})
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		start := time.Now()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				elapsed := time.Since(start).Round(time.Second)
				fmt.Printf(" ⏳ [%s] Running agents in parallel... (%s elapsed)\n", cmd.Args[0], elapsed)
			}
		}
	}()

	err := cmd.Run()
	close(done)
	return err
}
