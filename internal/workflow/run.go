package workflow

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"kv/internal/context"
	"kv/internal/review"
	"kv/internal/runner"
	"kv/internal/task"
	"kv/internal/workspace"
)

// runCommandOverride is a helper to override exec.Command in tests.
var runCommandOverride = exec.Command

// RunWorkflow executes the 7 phases of a workflow using OpenCode.
func RunWorkflow(workspaceDir, slug, userPrompt string) error {
	// 1. Ensure workflow directory exists (auto-create if not)
	workflowDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows", slug)
	if _, err := os.Stat(workflowDir); os.IsNotExist(err) {
		fmt.Printf("Workflow '%s' does not exist. Initializing structure...\n", slug)
		if err := NewWorkflow(workspaceDir, slug); err != nil {
			return fmt.Errorf("failed to initialize new workflow: %w", err)
		}
	}

	// Define phase titles and prompts/actions
	phases := []struct {
		Name   string
		Action func() error
	}{
		{
			Name: "Phase 1: Idea (Concepção)",
			Action: func() error {
				prompt := fmt.Sprintf(`You are in the IDEA phase of the workflow '%s'.
The user's objective is:
"%s"

Your task:
1. Explore the workspace directory to understand the current architecture and context.
2. Create or update the file '.kv/workflows/%s/idea.md' detailing:
   - Overview: A brief summary of the proposed feature or changes.
   - Problem Statement: What problem does this idea solve?
   - Proposed Solution: High-level description of the solution.
3. Do NOT modify any other files in the repository. Do NOT start implementing code.`, slug, userPrompt, slug)
				return runOpenCode(workspaceDir, prompt)
			},
		},
		{
			Name: "Phase 2: PRD (Product Requirements Document)",
			Action: func() error {
				prompt := fmt.Sprintf(`You are in the PRD phase of the workflow '%s'.
Your task:
1. Read '.kv/workflows/%s/idea.md'.
2. Create or update the file '.kv/workflows/%s/prd.md' detailing:
   - User Value: Why is this feature important to the user?
   - Key Requirements: A checklist of requirements (using markdown checkboxes '- [ ]').
   - Out of Scope: What is NOT covered by this workflow.
3. Do NOT modify any other files. Do NOT start implementing code.`, slug, slug, slug)
				return runOpenCode(workspaceDir, prompt)
			},
		},
		{
			Name: "Phase 3: Specs (Technical Specification)",
			Action: func() error {
				prompt := fmt.Sprintf(`You are in the SPECS phase of the workflow '%s'.
Your task:
1. Read '.kv/workflows/%s/prd.md'.
2. Inspect the codebase files that might be affected.
3. Create or update the file '.kv/workflows/%s/techspec.md' detailing:
   - Proposed Architecture: Details on the packages, data models, or APIs to modify/create.
   - Affected Components: List of components to change/create.
   - Validation Plan: Detailed commands, unit tests, or manual checks to verify the changes.
4. Define the tasks for implementation. Create markdown files for each task under '.kv/workflows/%s/tasks/' (e.g., '002-implement.md'). Each task file MUST contain YAML frontmatter:
   ---
   id: <task-id>
   title: <title>
   status: todo
   type: <feature|bug|chore>
   complexity: <low|medium|high>
   dependencies: [<dependency-task-ids>]
   agent/runner: opencode
   sources: [<affected-file-paths>]
   acceptanceCriteria:
     - <criterion-1>
   ---
   Followed by a description.
5. Do NOT modify any source code files yet.`, slug, slug, slug, slug)
				return runOpenCode(workspaceDir, prompt)
			},
		},
		{
			Name: "Phase 4: Implementation (Implementação)",
			Action: func() error {
				// Scan task files under .kv/workflows/<slug>/tasks/*.md
				tasksDir := filepath.Join(workflowDir, "tasks")
				files, err := ioutil.ReadDir(tasksDir)
				if err != nil {
					return fmt.Errorf("failed to read tasks directory: %w", err)
				}

				var taskIDs []string
				for _, f := range files {
					if !f.IsDir() && filepath.Ext(f.Name()) == ".md" {
						id := strings.TrimSuffix(f.Name(), ".md")
						// Skip setup task (001-setup) if it's already marked done or if we want to focus on feature tasks.
						taskFile := filepath.Join(tasksDir, f.Name())
						parsedTask, err := task.ParseTask(taskFile)
						if err == nil && parsedTask.Frontmatter.Status != "done" {
							taskIDs = append(taskIDs, id)
						}
					}
				}

				sort.Strings(taskIDs)

				if len(taskIDs) == 0 {
					fmt.Println("No pending tasks found for implementation.")
					return nil
				}

				fmt.Printf("Found %d pending task(s) to implement: %v\n", len(taskIDs), taskIDs)
				for _, taskID := range taskIDs {
					fmt.Printf("\n--- Implementing Task: %s ---\n", taskID)
					// Enrich task
					if err := task.EnrichTask(workspaceDir, slug, taskID); err != nil {
						return fmt.Errorf("failed to enrich task %s: %w", taskID, err)
					}
					// Build task context
					if err := context.BuildContext(workspaceDir, slug, taskID); err != nil {
						return fmt.Errorf("failed to build context for task %s: %w", taskID, err)
					}
					// Run task
					if err := runner.RunTask(workspaceDir, slug, taskID, "opencode", ""); err != nil {
						return fmt.Errorf("failed to execute implementation for task %s: %w", taskID, err)
					}
				}
				return nil
			},
		},
		{
			Name: "Phase 5: Review (Revisão)",
			Action: func() error {
				// Generate reviews for all tasks implemented in this workflow
				tasksDir := filepath.Join(workflowDir, "tasks")
				files, err := ioutil.ReadDir(tasksDir)
				if err != nil {
					return fmt.Errorf("failed to read tasks directory: %w", err)
				}

				var taskIDs []string
				for _, f := range files {
					if !f.IsDir() && filepath.Ext(f.Name()) == ".md" {
						id := strings.TrimSuffix(f.Name(), ".md")
						if id != "001-setup" {
							taskIDs = append(taskIDs, id)
						}
					}
				}

				sort.Strings(taskIDs)

				for _, taskID := range taskIDs {
					fmt.Printf("Generating review template for task %s...\n", taskID)
					if err := review.ReviewTask(workspaceDir, slug, taskID); err != nil {
						return fmt.Errorf("failed to create/process review for task %s: %w", taskID, err)
					}
				}

				// Ask OpenCode to run review verification
				prompt := fmt.Sprintf(`You are in the REVIEW phase of the workflow '%s'.
Your task:
1. Inspect the git diff and the changes made to the codebase.
2. Run any validation tests or check scripts specified in the Techspec or validation plan.
3. For each review file under '.kv/workflows/%s/reviews/*.review.md', verify the checklist items. Mark them as [x] once they are satisfied.
4. Fill in the review notes and decisions sections in the review file.
5. If there are any failing criteria or tests, do NOT approve yet.`, slug, slug)
				if err := runOpenCode(workspaceDir, prompt); err != nil {
					return err
				}

				// Recheck review files to see if they are approved
				for _, taskID := range taskIDs {
					fmt.Printf("Verifying review checklist for task %s...\n", taskID)
					_ = review.ReviewTask(workspaceDir, slug, taskID)
				}
				return nil
			},
		},
		{
			Name: "Phase 6: Adjustments (Ajustes)",
			Action: func() error {
				// Scan review files to see if any are still pending
				reviewsDir := filepath.Join(workflowDir, "reviews")
				files, err := ioutil.ReadDir(reviewsDir)
				if err != nil {
					// If no reviews folder/files, skip adjustments
					return nil
				}

				hasPending := false
				var pendingTasks []string
				for _, f := range files {
					if !f.IsDir() && filepath.Ext(f.Name()) == ".md" {
						reviewPath := filepath.Join(reviewsDir, f.Name())
						data, err := ioutil.ReadFile(reviewPath)
						if err == nil && strings.Contains(string(data), "status: pending-review") {
							hasPending = true
							taskID := strings.TrimSuffix(strings.TrimSuffix(f.Name(), ".review.md"), ".md")
							pendingTasks = append(pendingTasks, taskID)
						}
					}
				}

				if !hasPending {
					fmt.Println("All tasks have been approved! No adjustments needed.")
					return nil
				}

				fmt.Printf("The following tasks require adjustments: %v\n", pendingTasks)
				prompt := fmt.Sprintf(`You are in the ADJUSTMENTS phase of the workflow '%s'.
The following tasks are pending and have not passed review yet: %v.

Your task:
1. Read the pending review files and identify which acceptance criteria or tests failed.
2. Modify the source code to fix the issues.
3. Re-run tests to verify the fixes.
4. Once verified, update the checklists in the review files to [x].
5. Do NOT modify files outside allowed boundaries.`, slug, pendingTasks)
				if err := runOpenCode(workspaceDir, prompt); err != nil {
					return err
				}

				// Re-run review processors
				for _, taskID := range pendingTasks {
					_ = review.ReviewTask(workspaceDir, slug, taskID)
				}
				return nil
			},
		},
		{
			Name: "Phase 7: Memorize (Memorizar)",
			Action: func() error {
				prompt := fmt.Sprintf(`You are in the MEMORIZE phase of the workflow '%s'.
Your task:
1. Review the changes and lessons learned during implementation.
2. Complete/update the files under '.kv/workflows/%s/memory/' describing:
   - What went well.
   - What challenges were faced and how they were solved.
   - Any patterns or decisions that should be added to the Knowledge Vault.
3. If a Vault path is configured, promote/copy these markdown learnings to the vault under the appropriate category (e.g. 05-decisions or 07-runbooks).`, slug, slug)
				return runOpenCode(workspaceDir, prompt)
			},
		},
	}

	for _, phase := range phases {
		fmt.Printf("\n==================================================\n")
		fmt.Printf("   %s\n", phase.Name)
		fmt.Printf("==================================================\n")

		if err := phase.Action(); err != nil {
			return fmt.Errorf("failure during %s: %w", phase.Name, err)
		}
	}

	fmt.Println("\nWorkflow executed successfully from Idea to Memorize!")
	return nil
}

func runOpenCode(workspaceDir, prompt string) error {
	cmd := runCommandOverride("opencode", "run", prompt)
	cmd.Dir = workspaceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Run()
}
