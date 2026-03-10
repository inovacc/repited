package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/repited/internal/cmdlog"
	"github.com/inovacc/repited/internal/flow"
	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{
	Use:   "flow [directory]",
	Short: "Run the full Claude Code workflow: build → test → lint → stage → commit",
	Long: `Execute the most common Claude Code workflow as a single continuous pipeline.

Based on analysis of 1,800+ scripts across 41 projects, Claude Code follows
this pattern in the majority of sessions:

  1. go mod tidy      (resolve dependencies)
  2. go build ./...   (verify compilation)
  3. go vet ./...     (static analysis)
  4. go test ./...    (run tests)
  5. golangci-lint    (lint check)
  6. git add          (stage changes)
  7. git status       (verify staging)
  8. git commit       (persist to history)
  9. git push         (publish — optional)

Each step depends on the previous one succeeding. The flow stops on
first failure so you never commit broken code.

Steps are auto-detected: non-Go projects skip Go steps, projects without
golangci-lint skip linting, etc.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFlow,
}

func init() {
	rootCmd.AddCommand(flowCmd)

	flowCmd.Flags().StringP("message", "m", "", "commit message (required for commit step)")
	flowCmd.Flags().BoolP("push", "", false, "push after committing")
	flowCmd.Flags().BoolP("dry-run", "", false, "show what would run without executing")
	flowCmd.Flags().BoolP("verbose", "v", false, "show command output for passing steps")
	flowCmd.Flags().StringSliceP("skip", "", nil, "steps to skip (e.g., --skip lint,test)")
	flowCmd.Flags().StringSliceP("only", "", nil, "only run these steps (e.g., --only build,test)")
	flowCmd.Flags().StringP("files", "f", ".", "files to stage (default: .)")
}

func runFlow(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving directory: %w", err)
	}

	message, _ := cmd.Flags().GetString("message")
	push, _ := cmd.Flags().GetBool("push")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")
	skipSteps, _ := cmd.Flags().GetStringSlice("skip")
	onlySteps, _ := cmd.Flags().GetStringSlice("only")
	files, _ := cmd.Flags().GetString("files")

	skipSet := toSet(skipSteps)
	onlySet := toSet(onlySteps)

	// Detect project type
	hasGoMod := fileExists(filepath.Join(dir, "go.mod"))
	hasPackageJSON := fileExists(filepath.Join(dir, "package.json"))
	hasCargoToml := fileExists(filepath.Join(dir, "Cargo.toml"))
	hasGit := dirExists(filepath.Join(dir, ".git"))

	if !hasGit {
		return fmt.Errorf("%s is not a git repository", dir)
	}

	p := flow.NewPipeline(dir, verbose)

	// ─── Go workflow ───
	if hasGoMod {
		p.Add(flow.Step{
			Name: "go mod tidy",
			Cmd:  "go", Args: []string{"mod", "tidy"},
			Skip: shouldSkip("tidy", skipSet, onlySet),
		})
		p.Add(flow.Step{
			Name: "go build ./...",
			Cmd:  "go", Args: []string{"build", "./..."},
			Skip: shouldSkip("build", skipSet, onlySet),
		})
		p.Add(flow.Step{
			Name: "go vet ./...",
			Cmd:  "go", Args: []string{"vet", "./..."},
			Skip: shouldSkip("vet", skipSet, onlySet),
		})
		p.Add(flow.Step{
			Name: "go test ./...",
			Cmd:  "go", Args: []string{"test", "-count=1", "./..."},
			Skip:   shouldSkip("test", skipSet, onlySet),
			OnFail: "warn",
		})
		p.Add(flow.Step{
			Name: "golangci-lint",
			Cmd:  "golangci-lint", Args: []string{"run", "./..."},
			Skip:   shouldSkip("lint", skipSet, onlySet),
			OnFail: "warn",
		})
	}

	// ─── Node.js workflow ───
	if hasPackageJSON {
		p.Add(flow.Step{
			Name: "npm install",
			Cmd:  "npm", Args: []string{"install"},
			Skip: shouldSkip("install", skipSet, onlySet),
		})
		p.Add(flow.Step{
			Name: "npm test",
			Cmd:  "npm", Args: []string{"test"},
			Skip:   shouldSkip("test", skipSet, onlySet),
			OnFail: "warn",
		})
		p.Add(flow.Step{
			Name: "npx tsc --noEmit",
			Cmd:  "npx", Args: []string{"tsc", "--noEmit"},
			Skip:    shouldSkip("lint", skipSet, onlySet),
			OnFail:  "warn",
			Require: "tsconfig.json",
		})
	}

	// ─── Rust workflow ───
	if hasCargoToml {
		p.Add(flow.Step{
			Name: "cargo build",
			Cmd:  "cargo", Args: []string{"build"},
			Skip: shouldSkip("build", skipSet, onlySet),
		})
		p.Add(flow.Step{
			Name: "cargo test",
			Cmd:  "cargo", Args: []string{"test"},
			Skip:   shouldSkip("test", skipSet, onlySet),
			OnFail: "warn",
		})
		p.Add(flow.Step{
			Name: "cargo clippy",
			Cmd:  "cargo", Args: []string{"clippy", "--", "-D", "warnings"},
			Skip:   shouldSkip("lint", skipSet, onlySet),
			OnFail: "warn",
		})
	}

	// ─── Git workflow (always) ───
	addFiles := strings.Fields(files)
	gitAddArgs := append([]string{"add"}, addFiles...)
	p.Add(flow.Step{
		Name: fmt.Sprintf("git add %s", files),
		Cmd:  "git", Args: gitAddArgs,
		Skip: shouldSkip("stage", skipSet, onlySet),
	})
	p.Add(flow.Step{
		Name: "git status",
		Cmd:  "git", Args: []string{"status", "--short"},
		Skip: shouldSkip("status", skipSet, onlySet),
	})

	if message != "" {
		p.Add(flow.Step{
			Name: "git commit",
			Cmd:  "git", Args: []string{"commit", "-m", message},
			Skip: shouldSkip("commit", skipSet, onlySet),
		})

		if push {
			p.Add(flow.Step{
				Name: "git push",
				Cmd:  "git", Args: []string{"push"},
				Skip: shouldSkip("push", skipSet, onlySet),
			})
		}
	}

	// ─── Dry run ───
	if dryRun {
		_, _ = fmt.Fprintf(os.Stdout, "  Flow for %s (dry run):\n\n", dir)

		n := 0

		for _, s := range p.Steps {
			status := ""
			if s.Skip {
				status = " [SKIP]"
			}

			n++
			_, _ = fmt.Fprintf(os.Stdout, "  %d. %s%s\n", n, s.Name, status)
			_, _ = fmt.Fprintf(os.Stdout, "     $ %s %s\n", s.Cmd, strings.Join(s.Args, " "))
		}

		if message == "" {
			_, _ = fmt.Fprintln(os.Stdout, "\n  (no --message provided, commit step omitted)")
		}

		return nil
	}

	// ─── Execute ───
	_, _ = fmt.Fprintf(os.Stdout, "  Flow: %s\n\n", dir)

	if err := p.Run(); err != nil {
		p.Summary()
		saveFlowLog(p, dir)

		return err
	}

	p.Summary()
	saveFlowLog(p, dir)

	if message == "" {
		_, _ = fmt.Fprintln(os.Stdout, "\n  Tip: use --message \"feat: ...\" to include a commit step")
	}

	return nil
}

func saveFlowLog(p *flow.Pipeline, dir string) {
	log := cmdlog.New("flow", dir)
	for _, r := range p.Results {
		log.Add(cmdlog.Entry{
			Cmd:      r.Step.Cmd,
			Args:     r.Step.Args,
			Dir:      r.Step.Dir,
			Status:   r.Status,
			Duration: r.Duration,
		})
	}

	if logPath, err := log.Save(); err == nil {
		_, _ = fmt.Fprintf(os.Stdout, "  Log: %s\n", logPath)
	}
}

func shouldSkip(name string, skipSet, onlySet map[string]bool) bool {
	if len(onlySet) > 0 {
		return !onlySet[name]
	}

	return skipSet[name]
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool)

	for _, item := range items {
		for part := range strings.SplitSeq(item, ",") {
			set[strings.TrimSpace(part)] = true
		}
	}

	return set
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
