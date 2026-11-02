package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cooper-Wallace/repokit"
)

const version = "1.0.0"

func printHelp() {
	fmt.Printf(`repokit %s - Automate GitHub repository DevOps and Git hygiene standards.

Usage:
  repokit <command> [options]

Commands:
  create <name> [-d/--description] [-t/--type <python|java|typescript|bash|go>] [--public] [--path <dir>]
      Create a new standardized GitHub repository with automated DevOps scaffolding.

  configure [path] [-f/--force]
      Apply DevOps standards, branch rulesets, hooks, and CI workflows to an existing repo.

  audit [path] [-t/--type <python|java|typescript|bash|go>] [--local] [--remote]
      Audit repository compliance against remote GitHub settings and local standards.

  templates
      List available solution archetype templates.

  hook install [path] [--all]
      Install Conventional Commits (commit-msg) and pre-commit audit hooks.

  version, -v, --version
      Print repokit version.

  help, -h, --help
      Show this help message.
`, version)
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 {
		printHelp()
		return 1
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "help", "-h", "--help":
		printHelp()
		return 0

	case "version", "-v", "--version":
		fmt.Printf("repokit version %s\n", version)
		return 0

	case "templates":
		repokit.PrintTemplates()
		return 0

	case "create":
		var name, desc, templateType, targetDir string
		var public bool

		for i := 0; i < len(cmdArgs); i++ {
			arg := cmdArgs[i]
			switch arg {
			case "--public":
				public = true
			case "-d", "--description":
				if i+1 < len(cmdArgs) {
					i++
					desc = cmdArgs[i]
				}
			case "-t", "--type":
				if i+1 < len(cmdArgs) {
					i++
					templateType = cmdArgs[i]
				}
			case "--path":
				if i+1 < len(cmdArgs) {
					i++
					targetDir = cmdArgs[i]
				}
			default:
				if strings.HasPrefix(arg, "--description=") {
					desc = strings.TrimPrefix(arg, "--description=")
				} else if strings.HasPrefix(arg, "-d=") {
					desc = strings.TrimPrefix(arg, "-d=")
				} else if strings.HasPrefix(arg, "--type=") {
					templateType = strings.TrimPrefix(arg, "--type=")
				} else if strings.HasPrefix(arg, "-t=") {
					templateType = strings.TrimPrefix(arg, "-t=")
				} else if strings.HasPrefix(arg, "--path=") {
					targetDir = strings.TrimPrefix(arg, "--path=")
				} else if !strings.HasPrefix(arg, "-") && name == "" {
					name = arg
				}
			}
		}

		if name == "" {
			repokit.LogError("Repository name is required.")
			printHelp()
			return 1
		}

		if err := repokit.CreateRepository(name, desc, public, templateType, targetDir); err != nil {
			return 1
		}
		return 0

	case "configure":
		var path string
		var force bool

		for _, arg := range cmdArgs {
			if arg == "-f" || arg == "--force" {
				force = true
			} else if !strings.HasPrefix(arg, "-") && path == "" {
				path = arg
			}
		}

		if err := repokit.ConfigureRepository(path, force); err != nil {
			return 1
		}
		return 0

	case "audit":
		var path, templateType string
		var localOnly, remoteOnly bool

		for i := 0; i < len(cmdArgs); i++ {
			arg := cmdArgs[i]
			switch arg {
			case "--local":
				localOnly = true
			case "--remote":
				remoteOnly = true
			case "-t", "--type":
				if i+1 < len(cmdArgs) {
					i++
					templateType = cmdArgs[i]
				}
			default:
				if strings.HasPrefix(arg, "--type=") {
					templateType = strings.TrimPrefix(arg, "--type=")
				} else if strings.HasPrefix(arg, "-t=") {
					templateType = strings.TrimPrefix(arg, "-t=")
				} else if !strings.HasPrefix(arg, "-") && path == "" {
					path = arg
				}
			}
		}

		repoRoot, err := repokit.GetGitRepoRoot(path)
		if err != nil || repoRoot == "" {
			repokit.LogError("Not inside a git repository.")
			return 1
		}

		passed, err := repokit.AuditRepository(repoRoot, templateType, localOnly, remoteOnly)
		if err != nil || !passed {
			return 1
		}
		return 0

	case "hook":
		if len(cmdArgs) < 1 || cmdArgs[0] != "install" {
			repokit.LogError("Unknown hook command. Expected: repokit hook install [path] [--all]")
			return 1
		}
		hookArgs := cmdArgs[1:]
		var path string
		var all bool

		for _, arg := range hookArgs {
			if arg == "--all" {
				all = true
			} else if !strings.HasPrefix(arg, "-") && path == "" {
				path = arg
			}
		}

		repoRoot, err := repokit.GetGitRepoRoot(path)
		if err != nil || repoRoot == "" {
			repokit.LogError("Not inside a git repository.")
			return 1
		}

		if all {
			if err := repokit.InstallHooks(repoRoot, true); err != nil {
				return 1
			}
		} else {
			if err := repokit.InstallCommitHook(repoRoot); err != nil {
				return 1
			}
			repokit.LogInfo("Tip: Run 'repokit hook install --all' to also install the pre-commit audit hook.")
		}
		return 0

	default:
		repokit.LogError("Unknown command: %s", command)
		printHelp()
		return 1
	}
}

func init() {
	// Ensure executable path can be resolved cleanly
	_ = filepath.Base("")
}
