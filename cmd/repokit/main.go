package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Cooper-Wallace/repokit"
)

var version = repokit.Version

func printHelp() {
	fmt.Printf(`repokit %s - Automate GitHub repository DevOps and Git hygiene standards.

Usage:
  repokit <command> [options]

Commands:
  create <name> [-d/--description] [-t/--type <python|java|typescript|bash|go>] [--public] [--path <dir>]
      Create a new standardized GitHub repository with automated DevOps scaffolding.

  configure [path] [-f/--force] [--all] [--repos <r1,r2>] [--dry-run] [--pr] [--direct] [--release-ref <tag>] [--wait-for-merge] [--merge-timeout <dur>] [--concurrency <N>] [--offline]
      Apply DevOps standards, branch rulesets, hooks, and CI workflows to an existing repo (or across --all).

  audit [path] [-t/--type <python|java|typescript|bash|go>] [--local] [--remote] [--offline]
      Audit repository compliance against remote GitHub settings and local standards.

  templates
      List available solution archetype templates.

  hook install [path] [--all]
      Install Conventional Commits (commit-msg) and pre-commit audit hooks.

  release [path] [--dry-run] [--target-branch <main>] [--staging-branch <release-staging>]
      Stage a release PR (on release-staging) or promote/tag a release if HEAD is a release commit.

  update [--check] [--force] [--dry-run]
      Inspect and update repokit to the latest release from GitHub.

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
		var opts repokit.ConfigureOptions
		var path string

		for i := 0; i < len(cmdArgs); i++ {
			arg := cmdArgs[i]
			switch arg {
			case "-f", "--force":
				opts.Force = true
			case "--all":
				opts.All = true
			case "--dry-run":
				opts.DryRun = true
			case "--pr":
				opts.CreatePR = true
			case "--direct", "--push-main":
				opts.DirectCommit = true
			case "--release-ref":
				if i+1 < len(cmdArgs) {
					i++
					opts.ReleaseRef = cmdArgs[i]
				}
			case "--local":
				opts.Local = true
			case "--wait-for-merge":
				opts.WaitForMerge = true
			case "--merge-timeout":
				if i+1 < len(cmdArgs) {
					i++
					if d, err := time.ParseDuration(cmdArgs[i]); err == nil {
						opts.MergeTimeout = d
					}
				}
			case "--repos":
				if i+1 < len(cmdArgs) {
					i++
					reposStr := cmdArgs[i]
					for _, r := range strings.Split(reposStr, ",") {
						r = strings.TrimSpace(r)
						if r != "" {
							opts.Repos = append(opts.Repos, r)
						}
					}
				}
			case "--concurrency":
				if i+1 < len(cmdArgs) {
					i++
					if c, err := strconv.Atoi(cmdArgs[i]); err == nil {
						opts.Concurrency = c
					}
				}
			case "--offline":
				opts.Offline = true
			case "--path":
				if i+1 < len(cmdArgs) {
					i++
					path = cmdArgs[i]
				}
			default:
				if strings.HasPrefix(arg, "--concurrency=") {
					if c, err := strconv.Atoi(strings.TrimPrefix(arg, "--concurrency=")); err == nil {
						opts.Concurrency = c
					}
				} else if strings.HasPrefix(arg, "--merge-timeout=") {
					if d, err := time.ParseDuration(strings.TrimPrefix(arg, "--merge-timeout=")); err == nil {
						opts.MergeTimeout = d
					}
				} else if strings.HasPrefix(arg, "--release-ref=") {
					opts.ReleaseRef = strings.TrimPrefix(arg, "--release-ref=")
				} else if strings.HasPrefix(arg, "--repos=") {
					reposStr := strings.TrimPrefix(arg, "--repos=")
					for _, r := range strings.Split(reposStr, ",") {
						r = strings.TrimSpace(r)
						if r != "" {
							opts.Repos = append(opts.Repos, r)
						}
					}
				} else if strings.HasPrefix(arg, "--path=") {
					path = strings.TrimPrefix(arg, "--path=")
				} else if !strings.HasPrefix(arg, "-") && path == "" {
					path = arg
				}
			}
		}

		opts.TargetDir = path
		opts.LocalDir = path

		if err := repokit.ConfigureWithOptions(opts); err != nil {
			return 1
		}
		return 0

	case "audit":
		var path, templateType string
		var localOnly, remoteOnly, offline bool

		for i := 0; i < len(cmdArgs); i++ {
			arg := cmdArgs[i]
			switch arg {
			case "--local":
				localOnly = true
			case "--remote":
				remoteOnly = true
			case "--offline":
				offline = true
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

		if passed, err := repokit.AuditRepository(repoRoot, templateType, localOnly, remoteOnly, offline); err != nil || !passed {
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

	case "release":
		var path string
		var dryRun bool
		targetBranch := "main"
		stagingBranch := repokit.DefaultStagingBranch

		for i := 0; i < len(cmdArgs); i++ {
			arg := cmdArgs[i]
			switch arg {
			case "--dry-run":
				dryRun = true
			case "--target-branch":
				if i+1 < len(cmdArgs) {
					i++
					targetBranch = cmdArgs[i]
				}
			case "--staging-branch":
				if i+1 < len(cmdArgs) {
					i++
					stagingBranch = cmdArgs[i]
				}
			default:
				if strings.HasPrefix(arg, "--target-branch=") {
					targetBranch = strings.TrimPrefix(arg, "--target-branch=")
				} else if strings.HasPrefix(arg, "--staging-branch=") {
					stagingBranch = strings.TrimPrefix(arg, "--staging-branch=")
				} else if !strings.HasPrefix(arg, "-") && path == "" {
					path = arg
				}
			}
		}

		opts := repokit.ReleaseOptions{
			Path:          path,
			TargetBranch:  targetBranch,
			StagingBranch: stagingBranch,
			DryRun:        dryRun,
		}

		res, err := repokit.ExecuteRelease(opts)
		if err != nil {
			repokit.LogError("Release failed: %v", err)
			return 1
		}

		if res.ReleasesCreated {
			repokit.LogSuccess("Released %s successfully!", res.TagName)
		} else if res.Version != "" {
			repokit.LogSuccess("Release staged for %s (branch: %s)", res.TagName, stagingBranch)
		} else {
			repokit.LogInfo("No release needed.")
		}
		return 0

	case "update":
		var checkOnly, force, dryRun bool
		for i := 0; i < len(cmdArgs); i++ {
			arg := cmdArgs[i]
			switch arg {
			case "--check":
				checkOnly = true
			case "--force":
				force = true
			case "--dry-run":
				dryRun = true
			case "-h", "--help":
				fmt.Printf(`repokit update - Inspect and update repokit to the latest release.

Usage:
  repokit update [--check] [--force] [--dry-run]

Flags:
  --check    Inspect if a newer version is available without downloading or replacing.
  --force    Download and reinstall even if already on the latest version.
  --dry-run  Simulate the update process without writing files.
  -h, --help Show this help message.
`)
				return 0
			default:
				repokit.LogError("Unknown flag for update: %s", arg)
				return 1
			}
		}

		opts := repokit.UpdateOptions{
			CheckOnly: checkOnly,
			Force:     force,
			DryRun:    dryRun,
		}

		info, err := repokit.ExecuteSelfUpdate(opts)
		if err != nil {
			repokit.LogError("Update failed: %v", err)
			return 1
		}

		if checkOnly {
			if info.UpdateAvailable {
				repokit.LogInfo("A newer version of repokit is available: %s -> %s", info.CurrentVersion, info.LatestVersion)
				repokit.LogInfo("Run 'repokit update' to upgrade.")
			} else {
				repokit.LogSuccess("repokit is up to date (version %s).", info.CurrentVersion)
			}
			return 0
		}

		if dryRun {
			return 0
		}

		if !info.UpdateAvailable && !force {
			repokit.LogSuccess("repokit is already up to date (version %s).", info.CurrentVersion)
			return 0
		}

		repokit.LogSuccess("repokit updated successfully to version %s!", info.LatestVersion)
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
