package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"kv/internal/context"
	"kv/internal/fsutil"
	"kv/internal/opencode"
	"kv/internal/runner"
	"kv/internal/session"
	"kv/internal/task"
	"kv/internal/vault"
	"kv/internal/workflow"
	"kv/internal/workspace"
)

func main() {
	if len(os.Args) < 2 {
		printGeneralUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "help", "-h", "--help":
		printGeneralUsage()
		os.Exit(0)

	case "init":
		fs := flag.NewFlagSet("init", flag.ContinueOnError)
		vaultPathPtr := fs.String("vault", "", "Path to the Knowledge Vault")
		err := fs.Parse(os.Args[2:])
		if err != nil {
			os.Exit(1)
		}

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
			os.Exit(1)
		}

		cfg := &workspace.Config{
			VaultPath: "",
		}

		if *vaultPathPtr != "" {
			absVault, err := fsutil.ResolveAbs(*vaultPathPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to resolve vault path: %v\n", err)
				os.Exit(1)
			}
			if fsutil.IsDir(absVault) {
				_ = vault.EnsureVaultMarker(absVault)
			}
			if !vault.IsValidVault(absVault) {
				fmt.Fprintf(os.Stderr, "Error: '%s' is not a valid Knowledge Vault (missing .kv-vault file)\n", absVault)
				os.Exit(1)
			}
			relPath, err := filepath.Rel(cwd, absVault)
			if err != nil {
				cfg.VaultPath = absVault
			} else {
				cfg.VaultPath = filepath.Clean(relPath)
			}
		}

		err = workspace.SaveConfig(cwd, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to initialize workspace: %v\n", err)
			os.Exit(1)
		}

		// Create workflows directory
		workflowsDir := filepath.Join(cwd, workspace.ConfigDirName, "workflows")
		if err := os.MkdirAll(workflowsDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create workflows directory: %v\n", err)
			os.Exit(1)
		}

		// Write legacy marker and copy opencode templates
		_, _ = workspace.WriteMarker(cwd, cfg.VaultPath)
		_ = workspace.Init(cfg.VaultPath)

		fmt.Printf("Workspace initialized successfully under %s/.kv\n", cwd)
		if cfg.VaultPath != "" {
			fmt.Printf("Attached vault: %s\n", cfg.VaultPath)
		}

	case "find":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: Missing search query.")
			fmt.Fprintln(os.Stderr, "Usage: kv find <query>")
			os.Exit(1)
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
			os.Exit(1)
		}
		res, err := vault.FindVault(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		query := os.Args[2]
		results, err := vault.Search(res.Path, query, 10)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: search failed: %v\n", err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("Nenhum documento encontrado.")
			os.Exit(0)
		}

		fmt.Printf("Encontrados %d documento(s):\n\n", len(results))
		for i, r := range results {
			fmt.Printf("%d. %s\n", i+1, r.Title)
			fmt.Printf("   Arquivo: %s\n", r.File)
			fmt.Printf("   Score: %d\n", r.Score)
			if len(r.Tags) > 0 {
				fmt.Printf("   Tags: %s\n", strings.Join(r.Tags, ", "))
			}
			fmt.Printf("   Trecho: %s\n\n", r.Snippet)
		}

	case "context":
		if len(os.Args) < 3 {
			printContextUsage()
			os.Exit(1)
		}
		if os.Args[2] == "build" {
			if len(os.Args) < 5 {
				fmt.Fprintln(os.Stderr, "Error: Missing workflow slug or task ID.")
				fmt.Fprintln(os.Stderr, "Usage: kv context build <workflow-slug> <task-id>")
				os.Exit(1)
			}
			slug := os.Args[3]
			taskID := os.Args[4]
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			wsDir, err := workspace.FindWorkspaceDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			err = context.BuildContext(wsDir, slug, taskID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Context generated successfully under .opencode/context.md\n")
		} else {
			task := os.Args[2]
			err := vault.GenerateContext(task)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

	case "vault":
		if len(os.Args) < 3 {
			printVaultUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "attach":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: Missing vault path.")
				fmt.Fprintln(os.Stderr, "Usage: kv vault attach <path>")
				os.Exit(1)
			}
			vaultPath := os.Args[3]
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			wsDir, err := workspace.FindWorkspaceDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			absVault, err := fsutil.ResolveAbs(vaultPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if fsutil.IsDir(absVault) {
				_ = vault.EnsureVaultMarker(absVault)
			}
			if !vault.IsValidVault(absVault) {
				fmt.Fprintf(os.Stderr, "Error: '%s' is not a valid Knowledge Vault (missing .kv-vault file)\n", absVault)
				os.Exit(1)
			}

			cfg, err := workspace.LoadConfig(wsDir)
			if err != nil {
				cfg = &workspace.Config{}
			}

			relPath, err := filepath.Rel(wsDir, absVault)
			if err != nil {
				cfg.VaultPath = absVault
			} else {
				cfg.VaultPath = filepath.Clean(relPath)
			}

			err = workspace.SaveConfig(wsDir, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to save config: %v\n", err)
				os.Exit(1)
			}

			// Update legacy marker too
			_, _ = workspace.WriteMarker(wsDir, cfg.VaultPath)

			fmt.Printf("Vault attached successfully: %s\n", cfg.VaultPath)

		case "init":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: Missing path for vault init.")
				fmt.Fprintln(os.Stderr, "Usage: kv vault init <path>")
				os.Exit(1)
			}
			err := vault.Init(os.Args[3])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "path":
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(1)
			}
			res, err := vault.FindVault(cwd)
			if err != nil {
				if err.Error() == "Knowledge Vault not found" {
					printVaultNotFoundMessage()
				} else {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
				os.Exit(1)
			}
			fmt.Println(res.Path)
		case "doctor":
			healthy, err := vault.Doctor()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if !healthy {
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown vault subcommand '%s'\n", subCommand)
			printVaultUsage()
			os.Exit(1)
		}

	case "workspace":
		if len(os.Args) < 3 {
			printWorkspaceUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "init":
			fs := flag.NewFlagSet("workspace init", flag.ContinueOnError)
			vaultPathPtr := fs.String("vault", "", "Path to the Knowledge Vault")
			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(1)
			}

			// Initialize the new kv-workspace.yaml file
			wsName := filepath.Base(cwd)
			if wsName == "." || wsName == "/" {
				wsName = "workspace"
			}

			wsYaml := &workspace.WorkspaceYaml{
				Workspace: workspace.WorkspaceInfo{
					Name: wsName,
					Apps: []workspace.App{},
				},
			}

			// Check if file already exists
			if fsutil.IsFile(filepath.Join(cwd, workspace.WorkspaceYamlFileName)) {
				fmt.Print("File kv-workspace.yaml already exists. Overwrite? (y/N): ")
				var response string
				_, err = fmt.Scanln(&response)
				if err != nil || (strings.ToLower(strings.TrimSpace(response)) != "y" && strings.ToLower(strings.TrimSpace(response)) != "yes") {
					fmt.Println("Aborted.")
					os.Exit(0)
				}
			}

			err = workspace.SaveWorkspaceYaml(cwd, wsYaml)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to save workspace configuration: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Workspace configuration saved to %s/kv-workspace.yaml\n", cwd)

			// If legacy vault path is provided, also perform legacy workspace init
			if *vaultPathPtr != "" {
				err = workspace.Init(*vaultPathPtr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: legacy vault init failed: %v\n", err)
					os.Exit(1)
				}
				absVault, err := fsutil.ResolveAbs(*vaultPathPtr)
				if err == nil {
					relPath, err := filepath.Rel(cwd, absVault)
					var pathStr string
					if err != nil {
						pathStr = absVault
					} else {
						pathStr = filepath.Clean(relPath)
					}
					_ = workspace.SaveConfig(cwd, &workspace.Config{VaultPath: pathStr})
				}
			}

		case "show":
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(1)
			}
			wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			ws, err := workspace.LoadWorkspaceYaml(wsDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load workspace: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Workspace Name: %s\n", ws.Workspace.Name)
			fmt.Printf("Workspace Root: %s\n", wsDir)
			fmt.Printf("Applications (%d):\n", len(ws.Workspace.Apps))
			if len(ws.Workspace.Apps) == 0 {
				fmt.Println("  No applications registered.")
			} else {
				for _, app := range ws.Workspace.Apps {
					fmt.Printf("  - ID:    %s\n", app.ID)
					fmt.Printf("    Name:  %s\n", app.Name)
					fmt.Printf("    Path:  %s\n", app.Path)
					fmt.Printf("    Type:  %s\n", app.Type)
					fmt.Printf("    Stack: %s\n\n", app.Stack)
				}
			}

		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown workspace subcommand '%s'\n", subCommand)
			printWorkspaceUsage()
			os.Exit(1)
		}

	case "app":
		if len(os.Args) < 3 {
			printAppUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "list":
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(1)
			}
			wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			ws, err := workspace.LoadWorkspaceYaml(wsDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load workspace: %v\n", err)
				os.Exit(1)
			}

			if len(ws.Workspace.Apps) == 0 {
				fmt.Println("No applications registered in this workspace.")
				os.Exit(0)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tPATH\tTYPE\tSTACK")
			for _, app := range ws.Workspace.Apps {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", app.ID, app.Name, app.Path, app.Type, app.Stack)
			}
			w.Flush()

		case "add":
			fs := flag.NewFlagSet("app add", flag.ContinueOnError)
			idPtr := fs.String("id", "", "Application ID")
			namePtr := fs.String("name", "", "Application Name")
			pathPtr := fs.String("path", "", "Application Path")
			typePtr := fs.String("type", "", "Application Type")
			stackPtr := fs.String("stack", "", "Application Tech Stack")

			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}

			if *idPtr == "" || *namePtr == "" || *pathPtr == "" || *typePtr == "" || *stackPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: All fields (--id, --name, --path, --type, --stack) are required.")
				printAppUsage()
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(1)
			}
			wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ws, err := workspace.LoadWorkspaceYaml(wsDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load workspace: %v\n", err)
				os.Exit(1)
			}

			// Validate input path existence relative to current working directory
			absPath, err := fsutil.ResolveAbs(*pathPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to resolve path '%s': %v\n", *pathPtr, err)
				os.Exit(1)
			}
			if !fsutil.Exists(absPath) {
				fmt.Fprintf(os.Stderr, "Error: path '%s' does not exist\n", *pathPtr)
				os.Exit(1)
			}

			// Convert to relative path from workspace directory for portability
			relPath := fsutil.ResolveRel(wsDir, absPath)

			newApp := workspace.App{
				ID:    *idPtr,
				Name:  *namePtr,
				Path:  relPath,
				Type:  *typePtr,
				Stack: *stackPtr,
			}

			// Add to apps list
			ws.Workspace.Apps = append(ws.Workspace.Apps, newApp)

			// Validate overall configuration (checking duplicate IDs, etc.)
			err = ws.Validate(wsDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid application data: %v\n", err)
				os.Exit(1)
			}

			// Save workspace yaml
			err = workspace.SaveWorkspaceYaml(wsDir, ws)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to save workspace: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Application '%s' successfully added to the workspace.\n", *idPtr)

		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown app subcommand '%s'\n", subCommand)
			printAppUsage()
			os.Exit(1)
		}

	case "session":
		if len(os.Args) < 3 {
			printSessionUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "start":
			fs := flag.NewFlagSet("session start", flag.ContinueOnError)
			goalPtr := fs.String("goal", "", "Objective of the session")
			appsPtr := fs.String("apps", "", "Comma-separated list of application IDs")

			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}

			if *goalPtr == "" || *appsPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: Both --goal and --apps flags are required.")
				printSessionUsage()
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(1)
			}
			wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ws, err := workspace.LoadWorkspaceYaml(wsDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load workspace: %v\n", err)
				os.Exit(1)
			}

			// Parse apps list
			appIDs := strings.Split(*appsPtr, ",")
			for i := range appIDs {
				appIDs[i] = strings.TrimSpace(appIDs[i])
			}

			sess, err := session.StartSession(wsDir, ws, *goalPtr, appIDs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to start session: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Session started successfully!")
			fmt.Printf("  ID:            %s\n", sess.ID)
			fmt.Printf("  Goal:          %s\n", sess.Goal)
			fmt.Printf("  Selected Apps: %s\n", strings.Join(sess.SelectedApps, ", "))
			fmt.Println("  Allowed Paths:")
			for _, p := range sess.Boundary.AllowedPaths {
				fmt.Printf("    - %s\n", p)
			}
			fmt.Printf("  Session File:  .kv/sessions/%s/session.yaml\n", sess.ID)

		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown session subcommand '%s'\n", subCommand)
			printSessionUsage()
			os.Exit(1)
		}

	case "workflow":
		if len(os.Args) < 4 || os.Args[2] != "new" {
			printWorkflowUsage()
			os.Exit(1)
		}
		slug := os.Args[3]
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		wsDir, err := workspace.FindWorkspaceDir(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		err = workflow.NewWorkflow(wsDir, slug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Workflow '%s' created successfully under %s/.kv/workflows/%s\n", slug, wsDir, slug)

	case "task":
		if len(os.Args) < 3 {
			printTaskUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "enrich":
			if len(os.Args) < 5 {
				fmt.Fprintln(os.Stderr, "Error: Missing workflow slug or task ID.")
				fmt.Fprintln(os.Stderr, "Usage: kv task enrich <workflow-slug> <task-id>")
				os.Exit(1)
			}
			slug := os.Args[3]
			taskID := os.Args[4]
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			wsDir, err := workspace.FindWorkspaceDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			err = task.EnrichTask(wsDir, slug, taskID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Task '%s' enriched successfully. Context pack generated.\n", taskID)

		case "run":
			if len(os.Args) < 5 {
				fmt.Fprintln(os.Stderr, "Error: Missing workflow slug or task ID.")
				fmt.Fprintln(os.Stderr, "Usage: kv task run <workflow-slug> <task-id> [--runner <runner>]")
				os.Exit(1)
			}
			slug := os.Args[3]
			taskID := os.Args[4]

			fs := flag.NewFlagSet("task run", flag.ContinueOnError)
			runnerPtr := fs.String("runner", "opencode", "Runner type (e.g. opencode)")
			err := fs.Parse(os.Args[5:])
			if err != nil {
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			wsDir, err := workspace.FindWorkspaceDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			err = runner.RunTask(wsDir, slug, taskID, *runnerPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown task subcommand '%s'\n", subCommand)
			printTaskUsage()
			os.Exit(1)
		}

	case "opencode":
		if len(os.Args) < 3 {
			printOpenCodeUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "install":
			err := opencode.Install()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "doctor":
			healthy, err := opencode.Doctor()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if !healthy {
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown opencode subcommand '%s'\n", subCommand)
			printOpenCodeUsage()
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'\n", command)
		printGeneralUsage()
		os.Exit(1)
	}
}

func printGeneralUsage() {
	fmt.Println("kv - AI Development Harness")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  kv <command> [arguments]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  init [--vault <path>] Initialize workspace with .kv/config.yaml")
	fmt.Println("  find <query>          Search files inside the active vault")
	fmt.Println("  context build <workflow> <task-id>  Generate .opencode/context.md from context pack")
	fmt.Println("  vault                 Manage Knowledge Vault connections and creation")
	fmt.Println("  workspace             Manage workspace configuration (kv-workspace.yaml)")
	fmt.Println("  app                   Manage workspace registered applications")
	fmt.Println("  session               Manage multi-app development sessions")
	fmt.Println("  workflow new <slug>   Create a versionable workflow directory")
	fmt.Println("  task enrich <flow> <id>  Gather context, files, decisions and validation rules")
	fmt.Println("  task run <flow> <id>    Run task utilizing specified runner adapter")
	fmt.Println("  opencode              Install or inspect OpenCode agent commands integration")
	fmt.Println()
	fmt.Println("Use 'kv <command> --help' or 'kv <command> <subcommand>' for details.")
}

func printVaultUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv vault <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  attach <path> Attach an external Knowledge Vault")
	fmt.Println("  init <path>   Initialize a new Knowledge Vault structure at <path>")
	fmt.Println("  path          Discover and print the active vault path")
	fmt.Println("  doctor        Validate the health of the active vault")
}

func printWorkspaceUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv workspace <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  init [--vault <path>] Initialize workspace (creates kv-workspace.yaml)")
	fmt.Println("  show                  Display current workspace details")
}

func printAppUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv app <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  list                  List registered applications")
	fmt.Println("  add [flags]           Add a new application to the workspace")
	fmt.Println()
	fmt.Println("Flags for 'add':")
	fmt.Println("  --id <id>             Unique application ID")
	fmt.Println("  --name <name>         Application name")
	fmt.Println("  --path <path>         Path to application directory")
	fmt.Println("  --type <type>         Application type (e.g., backend, frontend)")
	fmt.Println("  --stack <stack>       Technology stack (e.g., go, typescript)")
}

func printSessionUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv session <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  start [flags]         Start a new multi-app development session")
	fmt.Println()
	fmt.Println("Flags for 'start':")
	fmt.Println("  --goal <goal>         The main objective/instruction for the session")
	fmt.Println("  --apps <app1,app2>    Comma-separated list of application IDs to include")
}

func printWorkflowUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv workflow new <slug>")
}

func printTaskUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv task <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  enrich <workflow-slug> <task-id> Compile context pack")
	fmt.Println("  run <workflow-slug> <task-id> [--runner <runner>] Execute task")
}

func printContextUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv context build <workflow-slug> <task-id>")
	fmt.Println("  kv context <legacy-task-query>")
}

func printOpenCodeUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv opencode <subcommand>")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  install    Install global scripts and templates")
	fmt.Println("  doctor     Validate OpenCode workspace configuration")
}

func printVaultNotFoundMessage() {
	fmt.Fprintln(os.Stderr, `Knowledge Vault not found.

Run one of:

kv vault init ./knowledge-vault
kv init --vault ./knowledge-vault

or attach an existing vault:

kv vault attach <path>`)
}
