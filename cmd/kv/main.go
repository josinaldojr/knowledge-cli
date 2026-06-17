package main

import (
	"flag"
	"fmt"
	"os"

	"kv/internal/vault"
	"kv/internal/workspace"
	"kv/internal/opencode"
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

	case "vault":
		if len(os.Args) < 3 {
			printVaultUsage()
			os.Exit(1)
		}
		subCommand := os.Args[2]
		switch subCommand {
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
			if *vaultPathPtr == "" {
				fmt.Fprintln(os.Stderr, "Error: Missing required --vault flag.")
				printWorkspaceUsage()
				os.Exit(1)
			}
			err = workspace.Init(*vaultPathPtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown workspace subcommand '%s'\n", subCommand)
			printWorkspaceUsage()
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
	fmt.Println("kv - Knowledge Vault CLI manager")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  kv <command> <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  vault       Manage Knowledge Vault structure and paths")
	fmt.Println("  workspace   Prepare or link your current workspace to a vault")
	fmt.Println("  opencode    Install or inspect OpenCode agent commands integration")
	fmt.Println()
	fmt.Println("Use 'kv <command>' for help on a specific command.")
}

func printVaultUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv vault <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  init <path>   Initialize a new Knowledge Vault structure at <path>")
	fmt.Println("  path          Discover and print the active vault path from current directory")
	fmt.Println("  doctor        Validate the health of the active vault")
}

func printWorkspaceUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv workspace init --vault <path>")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --vault <path>  The relative or absolute path of the target Knowledge Vault (must contain .kv-vault)")
}

func printOpenCodeUsage() {
	fmt.Println("Usage:")
	fmt.Println("  kv opencode <subcommand>")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  install    Install global scripts (kv-find.ps1) and markdown commands templates to ~/.config/opencode")
	fmt.Println("  doctor     Validate OpenCode workspace and commands configuration")
}

func printVaultNotFoundMessage() {
	fmt.Fprintln(os.Stderr, `Knowledge Vault not found.

Run one of:

kv vault init ./knowledge-vault
kv workspace init --vault ./knowledge-vault

or link an existing vault:

kv workspace init --vault <path>`)
}
