package main

import (
	stdcontext "context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	claudecodeadapter "kv/internal/adapter/claudecode"
	codexadapter "kv/internal/adapter/codex"
	"kv/internal/boundary"
	"kv/internal/context"
	"kv/internal/diff"
	"kv/internal/discovery"
	"kv/internal/fsutil"
	"kv/internal/lifecycle"
	"kv/internal/mcp"
	"kv/internal/memory"
	"kv/internal/opencode"
	"kv/internal/portable"
	"kv/internal/quality"
	"kv/internal/query"
	"kv/internal/report"
	"kv/internal/runner"
	"kv/internal/session"
	"kv/internal/snapshot"
	"kv/internal/spool"
	"kv/internal/store"
	"kv/internal/task"
	"kv/internal/tui"
	"kv/internal/vault"
	"kv/internal/wiki"
	"kv/internal/workflow"
	"kv/internal/workspace"

	"github.com/mattn/go-isatty"
)

func main() {
	if len(os.Args) < 2 {
		err := tui.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting TUI: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "mcp":
		if len(os.Args) > 2 {
			if err := runMCPAdmin(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if err := mcp.ServeStdio(); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printGeneralUsage()
		os.Exit(0)

	case "tui":
		err := tui.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting TUI: %v\n", err)
			os.Exit(1)
		}
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
			fs := flag.NewFlagSet("context build", flag.ContinueOnError)
			sessionPtr := fs.String("session", "", "Session ID to build context for")
			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if *sessionPtr != "" {
				wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				files, processed, warnings, err := context.BuildSessionContext(wsDir, *sessionPtr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				fmt.Println("Context compilation completed successfully!")
				fmt.Printf("Processed Applications: %d\n", processed)
				fmt.Println("Generated Files:")
				for _, f := range files {
					fmt.Printf("  - %s\n", f)
				}

				if len(warnings) > 0 {
					fmt.Println("\nWarnings:")
					for _, w := range warnings {
						fmt.Printf("  - %s\n", w)
					}
				}
			} else {
				args := fs.Args()
				var slug, taskID string
				if len(args) >= 1 {
					slug = args[0]
				}
				if len(args) >= 2 {
					taskID = args[1]
				}

				wsDir, err := workspace.FindWorkspaceDir(cwd)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				if slug == "" {
					if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
						selectedSlug, err := tui.PromptWorkflow(wsDir)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error: %v\n", err)
							os.Exit(1)
						}
						slug = selectedSlug
					} else {
						fmt.Fprintln(os.Stderr, "Error: Missing workflow slug.")
						printContextUsage()
						os.Exit(1)
					}
				}

				if taskID == "" {
					if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
						selectedTask, err := tui.PromptTask(wsDir, slug)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error: %v\n", err)
							os.Exit(1)
						}
						taskID = selectedTask
					} else {
						fmt.Fprintln(os.Stderr, "Error: Missing task ID.")
						printContextUsage()
						os.Exit(1)
					}
				}

				err = context.BuildContext(wsDir, slug, taskID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Context generated successfully under .opencode/context.md\n")
			}
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
			var needInit bool
			if err != nil {
				// Fallback to workspace yaml directory, or cwd
				wsDir, err = workspace.FindWorkspaceYamlDir(cwd)
				if err != nil {
					wsDir = cwd
				}
				needInit = true
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

			if needInit {
				// Initialize the workspace templates/workflows since we are doing it on the fly
				workflowsDir := filepath.Join(wsDir, workspace.ConfigDirName, "workflows")
				_ = os.MkdirAll(workflowsDir, 0755)
				_ = workspace.Init(cfg.VaultPath)
			}

			fmt.Printf("Vault attached successfully: %s\n", cfg.VaultPath)

		case "init":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: Missing path for vault init.")
				fmt.Fprintln(os.Stderr, "Usage: kv vault init <path>")
				os.Exit(1)
			}
			vaultPath := os.Args[3]
			err := vault.Init(vaultPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Automatically attach the vault to the current workspace after initialization
			cwd, err := os.Getwd()
			if err == nil {
				wsDir, errFind := workspace.FindWorkspaceDir(cwd)
				var needInit bool
				if errFind != nil {
					wsDir, errFind = workspace.FindWorkspaceYamlDir(cwd)
					if errFind != nil {
						wsDir = cwd
					}
					needInit = true
				}

				absVault, errResolve := fsutil.ResolveAbs(vaultPath)
				if errResolve == nil {
					cfg, errLoad := workspace.LoadConfig(wsDir)
					if errLoad != nil {
						cfg = &workspace.Config{}
					}

					relPath, errRel := filepath.Rel(wsDir, absVault)
					if errRel != nil {
						cfg.VaultPath = absVault
					} else {
						cfg.VaultPath = filepath.Clean(relPath)
					}

					errSave := workspace.SaveConfig(wsDir, cfg)
					if errSave == nil {
						_, _ = workspace.WriteMarker(wsDir, cfg.VaultPath)
						if needInit {
							workflowsDir := filepath.Join(wsDir, workspace.ConfigDirName, "workflows")
							_ = os.MkdirAll(workflowsDir, 0755)
							_ = workspace.Init(cfg.VaultPath)
						}
						fmt.Printf("Vault automatically attached to workspace: %s\n", cfg.VaultPath)
					}
				}
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
		case "status":
			if err := printGlobalWorkspaceStatus(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
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

		case "scan":
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(1)
			}
			detected, err := workspace.ScanWorkspaceApps(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning workspace: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Detected apps:")
			fmt.Println()
			for _, app := range detected {
				fmt.Printf("- %s\n  Path: %s\n  Stack: %s\n\n", app.ID, app.Path, app.Stack)
			}

			wsName := filepath.Base(cwd)
			if wsName == "." || wsName == "/" {
				wsName = "workspace"
			}
			wsYaml := &workspace.WorkspaceYaml{
				Workspace: workspace.WorkspaceInfo{
					Name: wsName,
					Apps: detected,
				},
			}
			err = workspace.SaveWorkspaceYaml(cwd, wsYaml)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error saving workspace configuration: %v\n", err)
			} else {
				fmt.Println("Workspace configuration saved to kv-workspace.yaml")
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

	case "boundary":
		if len(os.Args) < 3 {
			printBoundaryUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "validate":
			fs := flag.NewFlagSet("boundary validate", flag.ContinueOnError)
			sessionPtr := fs.String("session", "", "Session ID to validate")
			untrackedPtr := fs.Bool("include-untracked", false, "Include untracked files in validation")
			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}
			if *sessionPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: --session flag is required.")
				printBoundaryUsage()
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(2)
			}
			wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(2)
			}

			sess, err := session.LoadSession(wsDir, *sessionPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load session: %v\n", err)
				os.Exit(2)
			}

			report, err := boundary.ValidateSession(wsDir, sess, *untrackedPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: validation failed: %v\n", err)
				os.Exit(2)
			}

			boundary.PrintReport(report, wsDir)
			if report.IsValid() {
				os.Exit(0)
			} else {
				os.Exit(1)
			}

		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown boundary subcommand '%s'\n", subCommand)
			printBoundaryUsage()
			os.Exit(1)
		}

	case "session":
		if len(os.Args) < 3 {
			printSessionUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "list":
			if err := printGlobalSessions(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "init":
			fs := flag.NewFlagSet("session init", flag.ContinueOnError)
			idPtr := fs.String("id", "", "Session ID")
			goalPtr := fs.String("goal", "", "Objective of the session")
			appsPtr := fs.String("apps", "", "Comma-separated list of app_name=app_path")
			vaultPtr := fs.String("vault", "", "Comma-separated list of vault sources")
			writablePtr := fs.String("writable", "", "Comma-separated list of writable paths")
			agentPtr := fs.String("agent", "", "Agent name to use (e.g. backend, frontend)")

			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}

			if *goalPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: --goal flag is required.")
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(1)
			}
			wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
			if err != nil {
				wsDir = cwd
			}

			appsMap := make(map[string]string)
			if *appsPtr != "" {
				parts := strings.Split(*appsPtr, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					subParts := strings.SplitN(p, "=", 2)
					if len(subParts) == 2 {
						appsMap[strings.TrimSpace(subParts[0])] = strings.TrimSpace(subParts[1])
					} else {
						path := strings.TrimSpace(subParts[0])
						appsMap[filepath.Base(path)] = path
					}
				}
			}

			var vaultSources []string
			if *vaultPtr != "" {
				parts := strings.Split(*vaultPtr, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						vaultSources = append(vaultSources, p)
					}
				}
			}

			var writablePaths []string
			if *writablePtr != "" {
				parts := strings.Split(*writablePtr, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						writablePaths = append(writablePaths, p)
					}
				}
			}

			sess, err := session.InitSession(wsDir, *idPtr, *goalPtr, appsMap, vaultSources, writablePaths, *agentPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to initialize session: %v\n", err)
				os.Exit(1)
			}

			// Pre-create structures
			sessionDir := filepath.Join(wsDir, ".kv", "sessions", sess.ID)
			_ = os.WriteFile(filepath.Join(sessionDir, "context.md"), []byte(""), 0644)
			_ = os.WriteFile(filepath.Join(sessionDir, "opencode.md"), []byte(""), 0644)
			_ = os.WriteFile(filepath.Join(sessionDir, "audit.jsonl"), []byte(""), 0644)
			_ = os.WriteFile(filepath.Join(sessionDir, "report.md"), []byte(""), 0644)

			_ = session.LogEvent(wsDir, sess.ID, "session_created", nil)

			fmt.Println("Session initialized successfully!")
			fmt.Printf("  ID:            %s\n", sess.ID)
			fmt.Printf("  Goal:          %s\n", sess.Goal)
			fmt.Printf("  Session File:  .kv/sessions/%s/session.yaml\n", sess.ID)

		case "start":
			fs := flag.NewFlagSet("session start", flag.ContinueOnError)
			goalPtr := fs.String("goal", "", "Objective of the session")
			appsPtr := fs.String("apps", "", "Comma-separated list of application IDs")
			agentPtr := fs.String("agent", "", "Agent name to use (e.g. backend, frontend)")

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

			sess, err := session.StartSession(wsDir, ws, *goalPtr, appIDs, *agentPtr)
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

		case "validate":
			fs := flag.NewFlagSet("session validate", flag.ContinueOnError)
			sessionPtr := fs.String("session", "", "Session ID to validate")
			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}
			if *sessionPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: --session flag is required.")
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

			sess, err := session.LoadSession(wsDir, *sessionPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load session: %v\n", err)
				os.Exit(1)
			}

			err = boundary.ValidateSessionContract(wsDir, sess)
			if err != nil {
				_ = session.LogEvent(wsDir, sess.ID, "session_validated", map[string]interface{}{"status": "failed"})
				fmt.Fprintf(os.Stderr, "Session contract is invalid: %v\n", err)
				os.Exit(1)
			}

			_ = session.LogEvent(wsDir, sess.ID, "session_validated", map[string]interface{}{"status": "passed"})

			fmt.Println("Session valid")
			fmt.Println()
			fmt.Println("Allowed paths:")
			for _, p := range sess.Boundary.AllowedPaths {
				rel, err := filepath.Rel(wsDir, p)
				if err == nil {
					fmt.Printf("- ./%s\n", rel)
				} else {
					fmt.Printf("- %s\n", p)
				}
			}
			fmt.Println()
			fmt.Println("Writable paths:")
			for _, p := range sess.Boundary.WritablePaths {
				rel, err := filepath.Rel(wsDir, p)
				if err == nil {
					fmt.Printf("- ./%s\n", rel)
				} else {
					fmt.Printf("- %s\n", p)
				}
			}
			fmt.Println()
			fmt.Println("Readonly paths:")
			for _, p := range sess.Boundary.ReadonlyPaths {
				rel, err := filepath.Rel(wsDir, p)
				if err == nil {
					fmt.Printf("- ./%s\n", rel)
				} else {
					fmt.Printf("- %s\n", p)
				}
			}
			fmt.Println()
			fmt.Println("Policy:")
			fmt.Printf("- network: %v\n", sess.Policy.Network)
			fmt.Printf("- env read: %v\n", sess.Policy.AllowEnvRead)
			fmt.Printf("- delete files: %v\n", sess.Policy.AllowDeleteFiles)

		case "diff":
			fs := flag.NewFlagSet("session diff", flag.ContinueOnError)
			sessionPtr := fs.String("session", "", "Session ID to diff")
			untrackedPtr := fs.Bool("include-untracked", false, "Include untracked files in diff")
			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}
			if *sessionPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: --session flag is required.")
				printSessionUsage()
				os.Exit(1)
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
				os.Exit(2)
			}
			wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(2)
			}

			sess, err := session.LoadSession(wsDir, *sessionPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load session: %v\n", err)
				os.Exit(2)
			}

			report, err := boundary.ValidateSession(wsDir, sess, *untrackedPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: validation failed: %v\n", err)
				os.Exit(2)
			}

			if len(report.Allowed) == 0 {
				fmt.Println("No changes within session boundaries.")
			} else {
				var allowedPaths []string
				var untrackedPaths []string
				for _, val := range report.Allowed {
					rel, err := filepath.Rel(wsDir, val.Path)
					if err != nil {
						rel = val.Path
					}

					cmdCheck := exec.Command("git", "ls-files", "--error-unmatch", rel)
					cmdCheck.Dir = wsDir
					if err := cmdCheck.Run(); err != nil {
						untrackedPaths = append(untrackedPaths, rel)
					} else {
						allowedPaths = append(allowedPaths, rel)
					}
				}

				if len(allowedPaths) > 0 {
					cmdUnstaged := exec.Command("git", append([]string{"diff", "--"}, allowedPaths...)...)
					cmdUnstaged.Dir = wsDir
					cmdUnstaged.Stdout = os.Stdout
					cmdUnstaged.Stderr = os.Stderr
					_ = cmdUnstaged.Run()

					cmdStaged := exec.Command("git", append([]string{"diff", "--cached", "--"}, allowedPaths...)...)
					cmdStaged.Dir = wsDir
					cmdStaged.Stdout = os.Stdout
					cmdStaged.Stderr = os.Stderr
					_ = cmdStaged.Run()
				}

				for _, f := range untrackedPaths {
					fmt.Printf("\n--- /dev/null\n+++ b/%s\n", f)
					cmdUntracked := exec.Command("git", "diff", "--no-index", "--", "/dev/null", f)
					cmdUntracked.Dir = wsDir
					cmdUntracked.Stdout = os.Stdout
					cmdUntracked.Stderr = os.Stderr
					_ = cmdUntracked.Run()
				}
			}

			if len(report.Denied) > 0 || len(report.OutOfScope) > 0 {
				fmt.Fprintln(os.Stderr, "\nWARNING: There are changes violating session boundaries. Run 'kv boundary validate' for details.")
			}

		case "report":
			fs := flag.NewFlagSet("session report", flag.ContinueOnError)
			sessionPtr := fs.String("session", "", "Session ID to report")
			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}
			if *sessionPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: --session flag is required.")
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

			sess, err := session.LoadSession(wsDir, *sessionPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load session: %v\n", err)
				os.Exit(1)
			}

			// Generate quality gates on the fly
			qualityResults, qualityPassed, err := quality.RunQualityGates(wsDir, sess)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running quality gates: %v\n", err)
				os.Exit(1)
			}

			// Generate diff summary on the fly
			_, err = diff.GenerateDiffSummary(wsDir, sess)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating diff summary: %v\n", err)
				os.Exit(1)
			}

			err = report.GenerateSessionReport(wsDir, sess, qualityResults, qualityPassed)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Report successfully generated under .kv/sessions/%s/report.md\n", sess.ID)

		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown session subcommand '%s'\n", subCommand)
			printSessionUsage()
			os.Exit(1)
		}

	case "workflow":
		if len(os.Args) < 3 {
			printWorkflowUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "new":
			if len(os.Args) < 4 {
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

		case "run":
			var slug string
			var restArgs []string

			if len(os.Args) >= 4 && !strings.HasPrefix(os.Args[3], "-") {
				slug = os.Args[3]
				restArgs = os.Args[4:]
			} else {
				restArgs = os.Args[3:]
			}

			fs := flag.NewFlagSet("workflow run", flag.ContinueOnError)
			promptPtr := fs.String("prompt", "", "The goal or prompt to execute in this workflow")
			modelPtr := fs.String("model", "", "Model to execute (e.g. opencode/deepseek-v4-flash-free)")
			err := fs.Parse(restArgs)
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

			if slug == "" {
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					selectedSlug, err := tui.PromptWorkflow(wsDir)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}
					slug = selectedSlug
				} else {
					fmt.Fprintln(os.Stderr, "Error: Missing workflow slug.")
					fmt.Fprintln(os.Stderr, "Usage: kv workflow run <slug> --prompt <prompt> [--model <model>]")
					os.Exit(1)
				}
			}

			if *promptPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: Missing required --prompt flag.")
				fmt.Fprintln(os.Stderr, "Usage: kv workflow run <slug> --prompt <prompt> [--model <model>]")
				os.Exit(1)
			}

			if *modelPtr == "" {
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					selectedModel, err := tui.PromptModel()
					if err == nil && selectedModel != "" {
						*modelPtr = selectedModel
					}
				}
			}

			err = workflow.RunWorkflow(wsDir, slug, *promptPtr, *modelPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown workflow subcommand '%s'\n", subCommand)
			printWorkflowUsage()
			os.Exit(1)
		}

	case "task":
		if len(os.Args) < 3 {
			printTaskUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
		case "enrich":
			var slug string
			var taskID string

			if len(os.Args) >= 4 && !strings.HasPrefix(os.Args[3], "-") {
				slug = os.Args[3]
			}
			if len(os.Args) >= 5 && !strings.HasPrefix(os.Args[4], "-") {
				taskID = os.Args[4]
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

			if slug == "" {
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					selectedSlug, err := tui.PromptWorkflow(wsDir)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}
					slug = selectedSlug
				} else {
					fmt.Fprintln(os.Stderr, "Error: Missing workflow slug.")
					fmt.Fprintln(os.Stderr, "Usage: kv task enrich <workflow-slug> <task-id>")
					os.Exit(1)
				}
			}

			if taskID == "" {
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					selectedTask, err := tui.PromptTask(wsDir, slug)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}
					taskID = selectedTask
				} else {
					fmt.Fprintln(os.Stderr, "Error: Missing task ID.")
					fmt.Fprintln(os.Stderr, "Usage: kv task enrich <workflow-slug> <task-id>")
					os.Exit(1)
				}
			}

			err = task.EnrichTask(wsDir, slug, taskID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Task '%s' enriched successfully. Context pack generated.\n", taskID)

		case "run":
			var slug string
			var taskID string
			var restArgs []string

			argsIdx := 3
			var nonFlagArgs []string
			for argsIdx < len(os.Args) {
				arg := os.Args[argsIdx]
				if strings.HasPrefix(arg, "-") {
					break
				}
				nonFlagArgs = append(nonFlagArgs, arg)
				argsIdx++
			}
			restArgs = os.Args[argsIdx:]

			if len(nonFlagArgs) >= 1 {
				slug = nonFlagArgs[0]
			}
			if len(nonFlagArgs) >= 2 {
				taskID = nonFlagArgs[1]
			}

			fs := flag.NewFlagSet("task run", flag.ContinueOnError)
			runnerPtr := fs.String("runner", "opencode", "Runner type (e.g. opencode)")
			agentPtr := fs.String("agent", "", "Agent name to use (e.g. backend, frontend)")
			modelPtr := fs.String("model", "", "Model to execute (e.g. opencode/deepseek-v4-flash-free)")
			err := fs.Parse(restArgs)
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

			if slug == "" {
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					selectedSlug, err := tui.PromptWorkflow(wsDir)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}
					slug = selectedSlug
				} else {
					fmt.Fprintln(os.Stderr, "Error: Missing workflow slug.")
					fmt.Fprintln(os.Stderr, "Usage: kv task run <workflow-slug> <task-id> [--runner <runner>] [--agent <agent>] [--model <model>]")
					os.Exit(1)
				}
			}

			if taskID == "" {
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					selectedTask, err := tui.PromptTask(wsDir, slug)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
						os.Exit(1)
					}
					taskID = selectedTask
				} else {
					fmt.Fprintln(os.Stderr, "Error: Missing task ID.")
					fmt.Fprintln(os.Stderr, "Usage: kv task run <workflow-slug> <task-id> [--runner <runner>] [--agent <agent>] [--model <model>]")
					os.Exit(1)
				}
			}

			if *runnerPtr == "opencode" && *modelPtr == "" {
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					selectedModel, err := tui.PromptModel()
					if err == nil && selectedModel != "" {
						*modelPtr = selectedModel
					}
				}
			}

			err = runner.RunTask(wsDir, slug, taskID, *runnerPtr, *agentPtr, *modelPtr)
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
		case "uninstall":
			if err := opencode.Uninstall(); err != nil {
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

	case "change":
		if len(os.Args) != 3 || os.Args[2] != "history" {
			fmt.Fprintln(os.Stderr, "Usage: kv change history")
			os.Exit(1)
		}
		if err := printChangeHistory(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "memory":
		if len(os.Args) < 4 || os.Args[2] != "search" {
			fmt.Fprintln(os.Stderr, "Usage: kv memory search [--change <change-key>] <query>")
			os.Exit(1)
		}
		if err := printMemorySearch(os.Args[3:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "data":
		if err := runDataExchange(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "events":
		if len(os.Args) != 3 || os.Args[2] != "retry" {
			fmt.Fprintln(os.Stderr, "Usage: kv events retry")
			os.Exit(1)
		}
		if err := retryPendingEvents(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "run":
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		sessionPtr := fs.String("session", "", "Session ID to run")
		dryRunPtr := fs.Bool("dry-run", false, "Simulate execution without running the agent")
		agentPtr := fs.String("agent", "", "Agent name to override session agent config")
		modelPtr := fs.String("model", "", "Model to execute (e.g. opencode/deepseek-v4-flash-free)")
		err := fs.Parse(os.Args[2:])
		if err != nil {
			os.Exit(1)
		}

		if *sessionPtr == "" {
			fmt.Fprintln(os.Stderr, "Error: --session flag is required.")
			printGeneralUsage()
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

		if *modelPtr == "" && !*dryRunPtr {
			if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
				selectedModel, err := tui.PromptModel()
				if err == nil && selectedModel != "" {
					*modelPtr = selectedModel
				}
			}
		}

		err = runner.RunSession(wsDir, *sessionPtr, *agentPtr, *dryRunPtr, *modelPtr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running session: %v\n", err)
			os.Exit(1)
		}

	case "quality":
		if len(os.Args) < 3 || os.Args[2] != "run" {
			fmt.Fprintln(os.Stderr, "Usage: kv quality run --session <session-id>")
			os.Exit(1)
		}
		fs := flag.NewFlagSet("quality run", flag.ContinueOnError)
		sessionPtr := fs.String("session", "", "Session ID to run quality gates for")
		err := fs.Parse(os.Args[3:])
		if err != nil {
			os.Exit(1)
		}
		if *sessionPtr == "" {
			fmt.Fprintln(os.Stderr, "Error: --session flag is required.")
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

		sess, err := session.LoadSession(wsDir, *sessionPtr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading session: %v\n", err)
			os.Exit(1)
		}

		results, passed, err := quality.RunQualityGates(wsDir, sess)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running quality gates: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Quality Gates Results:")
		fmt.Println()
		for cmd, res := range results {
			fmt.Printf("- Command: `%s` -> %s\n", cmd, res)
		}
		fmt.Println()
		if passed {
			fmt.Println("Quality Gates: PASSED")
		} else {
			fmt.Println("Quality Gates: FAILED")
			os.Exit(1)
		}

	case "diff":
		if len(os.Args) >= 3 && os.Args[2] == "summarize" {
			fs := flag.NewFlagSet("diff summarize", flag.ContinueOnError)
			sessionPtr := fs.String("session", "", "Session ID to summarize diff for")
			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}
			if *sessionPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: --session flag is required.")
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

			sess, err := session.LoadSession(wsDir, *sessionPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading session: %v\n", err)
				os.Exit(1)
			}

			summary, err := diff.GenerateDiffSummary(wsDir, sess)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating diff summary: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(summary)
		} else {
			fmt.Fprintln(os.Stderr, "Usage: kv diff summarize --session <session-id>")
			os.Exit(1)
		}

	case "wiki":
		if len(os.Args) < 3 {
			printWikiUsage()
			os.Exit(1)
		}
		sub := os.Args[2]

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
			os.Exit(1)
		}
		res, err := vault.FindVault(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: active Knowledge Vault not found. Run 'kv vault init' or check your config.\n")
			os.Exit(1)
		}
		vaultPath := res.Path

		switch sub {
		case "help", "-h", "--help":
			printWikiUsage()
			os.Exit(0)

		case "compile":
			client, err := wiki.NewClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting up LLM Client: %v\n", err)
				fmt.Fprintln(os.Stderr, "Please ensure GEMINI_API_KEY environment variable is set.")
				os.Exit(1)
			}
			fmt.Println("Iniciando compilação de notas brutas...")
			count, err := wiki.CompileInbox(vaultPath, client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro na compilação: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Compilação concluída. %d arquivo(s) processado(s).\n", count)

		case "link":
			client, err := wiki.NewClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting up LLM Client: %v\n", err)
				fmt.Fprintln(os.Stderr, "Please ensure GEMINI_API_KEY environment variable is set.")
				os.Exit(1)
			}
			err = wiki.AutoLinkAll(vaultPath, client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro ao gerar links: %v\n", err)
				os.Exit(1)
			}

		case "ask":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: Missing query.")
				fmt.Fprintln(os.Stderr, "Usage: kv wiki ask \"<pergunta>\"")
				os.Exit(1)
			}
			query := os.Args[3]

			client, err := wiki.NewClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting up LLM Client: %v\n", err)
				fmt.Fprintln(os.Stderr, "Please ensure GEMINI_API_KEY environment variable is set.")
				os.Exit(1)
			}

			fmt.Println("Consultando a LLM Wiki...")
			answer, err := wiki.AskWiki(vaultPath, client, query)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro ao consultar a wiki: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("\nResposta da Wiki:")
			fmt.Println("--------------------")
			fmt.Println(answer)
			fmt.Println("--------------------")

		case "serve":
			fs := flag.NewFlagSet("wiki serve", flag.ContinueOnError)
			portPtr := fs.Int("port", 8080, "Port to run the local web server on")
			err := fs.Parse(os.Args[3:])
			if err != nil {
				os.Exit(1)
			}

			client, err := wiki.NewClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting up LLM Client: %v\n", err)
				fmt.Fprintln(os.Stderr, "Please ensure GEMINI_API_KEY environment variable is set.")
				os.Exit(1)
			}

			srv := wiki.NewServer(vaultPath, client, *portPtr)
			err = srv.Start()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro ao iniciar o servidor: %v\n", err)
				os.Exit(1)
			}

		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown wiki subcommand '%s'\n", sub)
			printWikiUsage()
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'\n", command)
		printGeneralUsage()
		os.Exit(1)
	}
}

func withGlobalStore(fn func(store.Paths, *store.Repository) error) error {
	paths, err := store.ResolvePaths()
	if err != nil {
		return err
	}
	database, err := store.Open(stdcontext.Background(), paths.DatabasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := database.Migrate(stdcontext.Background(), paths.DatabasePath); err != nil {
		return err
	}
	return fn(paths, store.NewRepository(database))
}

func currentWorkspaceID(ctx stdcontext.Context, repository *store.Repository) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	identity, err := discovery.DeriveIdentity(cwd)
	if err != nil {
		return "", err
	}
	workspaceID, err := repository.WorkspaceID(ctx, identity.LogicalID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("no global KV workspace record for this directory")
	}
	return workspaceID, err
}

func runMCPAdmin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kv mcp <status|retry|reconcile|doctor>")
	}
	switch args[0] {
	case "status":
		return printMCPStatus()
	case "retry":
		return retryPendingEvents()
	case "reconcile":
		fs := flag.NewFlagSet("mcp reconcile", flag.ContinueOnError)
		timeout := fs.Duration("timeout", 24*time.Hour, "Mark inactive sessions and pending operations stale after this duration")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return reconcileStale(*timeout)
	case "doctor":
		fs := flag.NewFlagSet("mcp doctor", flag.ContinueOnError)
		provider := fs.String("provider", "all", "Provider to inspect: opencode, claude-code, codex, or all")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return printProviderDoctor(*provider)
	default:
		return fmt.Errorf("unknown mcp subcommand %q", args[0])
	}
}

func printMCPStatus() error {
	return withGlobalStore(func(paths store.Paths, _ *store.Repository) error {
		queue, err := spool.Open(paths.SpoolDir)
		if err != nil {
			return err
		}
		entries, err := queue.Entries()
		if err != nil {
			return err
		}
		fmt.Println("MCP status: ready")
		fmt.Printf("Global data: %s\n", paths.DataDir)
		fmt.Printf("Database: %s\n", paths.DatabasePath)
		fmt.Printf("Pending events: %d\n", len(entries))
		return nil
	})
}

func printGlobalWorkspaceStatus() error {
	return withGlobalStore(func(_ store.Paths, repository *store.Repository) error {
		workspaceID, err := currentWorkspaceID(stdcontext.Background(), repository)
		if err != nil {
			return err
		}
		status, err := repository.GetWorkspaceStatus(stdcontext.Background(), workspaceID)
		if err != nil {
			return err
		}
		fmt.Printf("Workspace: %s\n", status.Identity)
		fmt.Printf("Workspace ID: %s\n", status.ID)
		fmt.Printf("Active sessions: %d\n", status.ActiveSessions)
		fmt.Printf("OpenSpec changes: %d\n", status.Changes)
		return nil
	})
}

func printGlobalSessions() error {
	return withGlobalStore(func(_ store.Paths, repository *store.Repository) error {
		workspaceID, err := currentWorkspaceID(stdcontext.Background(), repository)
		if err != nil {
			return err
		}
		sessions, err := repository.CurrentSessions(stdcontext.Background(), workspaceID)
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			fmt.Println("No global provider sessions.")
			return nil
		}
		writer := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(writer, "ID\tPROVIDER\tSTATUS\tLAST SEEN")
		for _, session := range sessions {
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", session.ID, session.Provider, session.Status, session.LastSeenAt.Format(time.RFC3339))
		}
		return writer.Flush()
	})
}

func printChangeHistory() error {
	return withGlobalStore(func(_ store.Paths, repository *store.Repository) error {
		workspaceID, err := currentWorkspaceID(stdcontext.Background(), repository)
		if err != nil {
			return err
		}
		changes, err := repository.ChangeHistory(stdcontext.Background(), workspaceID)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			fmt.Println("No OpenSpec change history.")
			return nil
		}
		for _, change := range changes {
			fmt.Printf("%s\t%s\n", change.Key, change.Status)
		}
		return nil
	})
}

func printMemorySearch(args []string) error {
	fs := flag.NewFlagSet("memory search", flag.ContinueOnError)
	change := fs.String("change", "", "Limit results to an OpenSpec change key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	text := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if text == "" {
		return fmt.Errorf("memory search query is required")
	}
	return withGlobalStore(func(_ store.Paths, repository *store.Repository) error {
		workspaceID, err := currentWorkspaceID(stdcontext.Background(), repository)
		if err != nil {
			return err
		}
		results, err := query.New(memory.NewRetriever(repository), repository).Memories(stdcontext.Background(), memory.Query{WorkspaceID: workspaceID, ChangeKey: *change, Text: text, Limit: 20})
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("No matching engineering memory.")
			return nil
		}
		for _, result := range results {
			fmt.Printf("[%s] %s\n  Change: %s\n  Source: %s @ %s\n", result.Kind, result.Summary, result.ChangeKey, result.ArtifactPath, result.RevisionID)
		}
		return nil
	})
}

func runDataExchange(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kv data <export|import>")
	}
	fs := flag.NewFlagSet("data "+args[0], flag.ContinueOnError)
	file := fs.String("file", "", "Path to a KV knowledge JSON document")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *file == "" || len(fs.Args()) != 0 {
		return fmt.Errorf("--file is required")
	}
	switch args[0] {
	case "export":
		return exportKnowledge(*file)
	case "import":
		return importKnowledge(*file)
	default:
		return fmt.Errorf("unknown data subcommand %q", args[0])
	}
}

func exportKnowledge(path string) error {
	return withGlobalStore(func(_ store.Paths, repository *store.Repository) error {
		workspaceID, err := currentWorkspaceID(stdcontext.Background(), repository)
		if err != nil {
			return err
		}
		document, err := repository.ExportKnowledge(stdcontext.Background(), workspaceID)
		if err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("create export file: %w", err)
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(document); err != nil {
			return fmt.Errorf("write export: %w", err)
		}
		fmt.Printf("Exported global KV knowledge to %s (%d sessions, %d memories)\n", path, len(document.Sessions), len(document.Memories))
		return nil
	})
}

func importKnowledge(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open import file: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var document portable.Document
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode import: %w", err)
	}
	if err := document.Validate(); err != nil {
		return fmt.Errorf("invalid import: %w", err)
	}
	return withGlobalStore(func(_ store.Paths, repository *store.Repository) error {
		if err := repository.ImportKnowledge(stdcontext.Background(), document); err != nil {
			return fmt.Errorf("import knowledge: %w", err)
		}
		fmt.Printf("Imported global KV knowledge for %s (%d sessions, %d memories)\n", document.Workspace.Identity, len(document.Sessions), len(document.Memories))
		return nil
	})
}

func retryPendingEvents() error {
	return withGlobalStore(func(paths store.Paths, repository *store.Repository) error {
		queue, err := spool.Open(paths.SpoolDir)
		if err != nil {
			return err
		}
		entries, err := queue.Due(time.Now().UTC())
		if err != nil {
			return err
		}
		snapshots, err := snapshot.Open(paths.SnapshotDir, 1<<20, nil)
		if err != nil {
			return err
		}
		hooks := lifecycle.NewHookApplicationService(lifecycle.NewSessionService(repository), repository, snapshot.NewCapturer(snapshots, repository)).WithMemoryRetriever(memory.NewRetriever(repository)).WithArchiveConsolidator(memory.NewConsolidator(repository))
		var delivered, deferred int
		for _, entry := range entries {
			if _, err := hooks.Handle(stdcontext.Background(), entry.Envelope); err != nil {
				if err := queue.Retry(entry, time.Now().UTC()); err != nil {
					return err
				}
				deferred++
				continue
			}
			if err := queue.Acknowledge(entry.Envelope.EventID); err != nil {
				return err
			}
			delivered++
		}
		fmt.Printf("Retried pending events: delivered=%d deferred=%d\n", delivered, deferred)
		return nil
	})
}

func reconcileStale(timeout time.Duration) error {
	return withGlobalStore(func(_ store.Paths, repository *store.Repository) error {
		sessions, operations, err := lifecycle.NewSessionService(repository).Reconcile(stdcontext.Background(), timeout, time.Now().UTC())
		if err != nil {
			return err
		}
		fmt.Printf("Reconciled stale state: sessions=%d operations=%d\n", sessions, operations)
		return nil
	})
}

func printProviderDoctor(provider string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	providers := []string{provider}
	if provider == "all" {
		providers = []string{"opencode", "claude-code", "codex"}
	}
	for _, name := range providers {
		var binary, configured, lifecycleAdapter, spoolRecovery bool
		switch name {
		case "opencode":
			report := opencode.CheckMCP(filepath.Join(home, ".config", "opencode"), "kv")
			binary, configured, lifecycleAdapter, spoolRecovery = report.Binary, report.MCPConfigured, report.LifecycleAdapter, report.SpoolRecovery
		case "claude-code":
			report := claudecodeadapter.CheckMCP(filepath.Join(home, ".claude", "settings.json"), "claude")
			binary, configured, lifecycleAdapter, spoolRecovery = report.Binary, report.MCPConfigured, report.LifecycleAdapter, report.SpoolRecovery
		case "codex":
			report := codexadapter.CheckMCP(filepath.Join(home, ".codex", "config.toml"), "codex")
			binary, configured, lifecycleAdapter, spoolRecovery = report.Binary, report.MCPConfigured, report.LifecycleAdapter, report.SpoolRecovery
		default:
			return fmt.Errorf("unsupported provider %q", name)
		}
		status := "unhealthy"
		if binary && configured && lifecycleAdapter && spoolRecovery {
			status = "healthy"
		}
		fmt.Printf("%s: %s (binary=%t mcp_configured=%t lifecycle_adapter=%t spool_recovery=%t)\n", name, status, binary, configured, lifecycleAdapter, spoolRecovery)
	}
	return nil
}

func printGeneralUsage() {
	fmt.Println("kv - AI Development Harness")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  kv <command> [arguments]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  tui                   Launch the interactive Terminal User Interface (default)")
	fmt.Println("  mcp                   Start the KV MCP server over stdio")
	fmt.Println("  mcp status|retry|reconcile|doctor  Administer global MCP state")
	fmt.Println("  init [--vault <path>] Initialize workspace with .kv/config.yaml")
	fmt.Println("  find <query>          Search files inside the active vault")
	fmt.Println("  context build <workflow> <task-id>  Generate .opencode/context.md from context pack")
	fmt.Println("  vault                 Manage Knowledge Vault connections and creation")
	fmt.Println("  workspace             Manage workspace configuration (kv-workspace.yaml)")
	fmt.Println("  change history        Show global OpenSpec change history")
	fmt.Println("  memory search <query> Search global engineering memory")
	fmt.Println("  data export|import    Exchange global session metadata and evidence-linked memory as JSON")
	fmt.Println("  events retry          Retry due global lifecycle events")
	fmt.Println("  app                   Manage workspace registered applications")
	fmt.Println("  session               Manage multi-app development sessions")
	fmt.Println("  boundary              Validate session boundaries")
	fmt.Println("  workflow              Manage versionable workflows (new, run)")
	fmt.Println("  task enrich <flow> <id>  Gather context, files, decisions and validation rules")
	fmt.Println("  task run <flow> <id>    Run task utilizing specified runner adapter")
	fmt.Println("  opencode              Install or inspect OpenCode agent commands integration")
	fmt.Println("  wiki                  Manage the LLM Wiki (compile notes, link pages, ask questions)")
	fmt.Println()
	fmt.Println("Use 'kv <command> --help' or 'kv <command> <subcommand>' for details.")
}

func printWikiUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv wiki <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  compile     Scan 00-inbox/ and compile notes using LLM into canonical directories")
	fmt.Println("  link        Run a pass over all canonical wiki files to generate relative cross-links")
	fmt.Println("  ask <query> Ask a natural language question to the compiled wiki")
	fmt.Println("  serve       Launch the local premium web interface on a specified port")
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
	fmt.Println("  diff [flags]          Show git diff of session allowed paths")
	fmt.Println()
	fmt.Println("Flags for 'start':")
	fmt.Println("  --goal <goal>         The main objective/instruction for the session")
	fmt.Println("  --apps <app1,app2>    Comma-separated list of application IDs to include")
	fmt.Println()
	fmt.Println("Flags for 'diff':")
	fmt.Println("  --session <id>        Session ID to compare")
	fmt.Println("  --include-untracked   Include untracked files in diff")
}

func printBoundaryUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv boundary <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  validate [flags]      Validate changed files against session boundaries")
	fmt.Println()
	fmt.Println("Flags for 'validate':")
	fmt.Println("  --session <id>        Session ID to validate")
	fmt.Println("  --include-untracked   Include untracked files in validation")
}

func printWorkflowUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv workflow <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  new <slug>                      Create a versionable workflow directory")
	fmt.Println("  run <slug> --prompt <prompt>    Run the 7-phase orchestrated workflow using OpenCode")
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
	fmt.Println("  kv context build --session <session-id>    Compile session context")
	fmt.Println("  kv context build <workflow-slug> <task-id> Compile task context (legacy)")
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
