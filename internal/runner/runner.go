package runner

import (
	"fmt"

	"kv/internal/context"
	"kv/internal/opencode"
)

// RunTask executes a task using the specified runner.
func RunTask(workspaceDir, workflowSlug, taskID, runnerType string) error {
	// 1. Build the context first to ensure .opencode/context.md is generated
	fmt.Printf("Building context for task %s in workflow %s...\n", taskID, workflowSlug)
	if err := context.BuildContext(workspaceDir, workflowSlug, taskID); err != nil {
		return fmt.Errorf("failed to build context pack before running task: %v", err)
	}

	// 2. Resolve runner type
	if runnerType == "" {
		runnerType = "opencode"
	}

	switch runnerType {
	case "opencode":
		fmt.Printf("Executing task using runner: %s\n", runnerType)
		// Check if opencode is installed/healthy
		healthy, err := opencode.Doctor()
		if err != nil || !healthy {
			fmt.Println("Warning: OpenCode configuration seems unhealthy. Run 'kv opencode doctor' or 'kv opencode install' to fix.")
		}

		fmt.Println("\nOpenCode Runner Initialized successfully.")
		fmt.Printf("The task is now configured. OpenCode will automatically read '.opencode/context.md' inside your workspace to perform changes.\n")
		return nil

	default:
		return fmt.Errorf("unsupported runner type: '%s'. Supported runners: opencode", runnerType)
	}
}
