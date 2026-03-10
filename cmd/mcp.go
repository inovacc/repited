package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	mcpserver "github.com/inovacc/repited/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server management",
	Long:  `Manage the repited MCP (Model Context Protocol) server for Claude Code integration.`,
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the MCP server on stdio",
	Long: `Start the repited MCP server using stdio transport.
This is invoked automatically by Claude Code — you don't need to run this manually.

Exposed tools:
  flow       — Run build → test → lint → stage → commit pipeline
  scan       — Scan directories for .scripts folders
  stats      — Query stored scan statistics
  relations  — Analyze command co-occurrence patterns`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcpserver.Serve(cmd.Context(), Version)
	},
}

var mcpInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install repited as an MCP server for Claude Code",
	Long: `Register repited in Claude Code's MCP settings.

Use --global to write to ~/.claude.json (global) or --project to write to .mcp.json (project-level).

After install, Claude Code will have access to these tools:
  flow       — Run full development workflow (build, test, lint, commit)
  scan       — Discover and analyze .scripts folders
  stats      — Query scan data from SQLite
  relations  — Show command co-occurrence and sequencing patterns

Usage:
  go install github.com/inovacc/repited@latest
  repited mcp install --global --client claude
  repited mcp install --project --client claude`,
	RunE: runMCPInstall,
}

var mcpUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove repited from Claude Code's MCP settings",
	RunE:  runMCPUninstall,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpInstallCmd)
	mcpCmd.AddCommand(mcpUninstallCmd)

	mcpInstallCmd.Flags().BoolP("global", "g", false, "install globally (~/.claude.json)")
	mcpInstallCmd.Flags().BoolP("project", "p", false, "install for this project (.mcp.json)")
	mcpInstallCmd.Flags().StringP("client", "c", "claude", "target client (claude)")
	mcpInstallCmd.MarkFlagsMutuallyExclusive("global", "project")

	mcpUninstallCmd.Flags().BoolP("global", "g", false, "uninstall globally")
	mcpUninstallCmd.Flags().StringP("client", "c", "claude", "target client (claude)")
}

func runMCPInstall(cmd *cobra.Command, _ []string) error {
	global, _ := cmd.Flags().GetBool("global")
	project, _ := cmd.Flags().GetBool("project")
	client, _ := cmd.Flags().GetString("client")

	if !global && !project {
		return fmt.Errorf("specify --global or --project to choose the install scope")
	}

	if client != "claude" {
		return fmt.Errorf("unsupported client %q (only 'claude' is supported)", client)
	}

	// Find the repited binary
	binaryPath, err := findBinary()
	if err != nil {
		return fmt.Errorf("finding repited binary: %w", err)
	}

	if project {
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		if err := mcpserver.InstallProject(binaryPath, projectDir); err != nil {
			return fmt.Errorf("installing MCP server: %w", err)
		}

		_, _ = fmt.Fprintln(os.Stdout, "Installed repited MCP server for this project (.mcp.json)")
		_, _ = fmt.Fprintf(os.Stdout, "  Binary: %s\n", binaryPath)
		_, _ = fmt.Fprintf(os.Stdout, "  Config: .mcp.json\n")
		_, _ = fmt.Fprintf(os.Stdout, "  Tools:  flow, scan, stats, relations\n")
		_, _ = fmt.Fprintln(os.Stdout, "\nRestart Claude Code to activate.")

		return nil
	}

	if err := mcpserver.InstallGlobal(binaryPath); err != nil {
		return fmt.Errorf("installing MCP server: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, "Installed repited MCP server globally for Claude Code")
	_, _ = fmt.Fprintf(os.Stdout, "  Binary: %s\n", binaryPath)
	_, _ = fmt.Fprintf(os.Stdout, "  Config: ~/.claude.json\n")
	_, _ = fmt.Fprintf(os.Stdout, "  Tools:  flow, scan, stats, relations\n")
	_, _ = fmt.Fprintln(os.Stdout, "\nRestart Claude Code to activate.")

	return nil
}

func runMCPUninstall(cmd *cobra.Command, _ []string) error {
	client, _ := cmd.Flags().GetString("client")

	if client != "claude" {
		return fmt.Errorf("unsupported client %q", client)
	}

	if err := mcpserver.UninstallGlobal(); err != nil {
		return fmt.Errorf("uninstalling MCP server: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, "Removed repited from Claude Code's MCP settings.")

	return nil
}

// findBinary locates the repited executable.
func findBinary() (string, error) {
	// First: check if running from a compiled binary
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
		if err == nil {
			// Ensure it's not a temp binary from `go run`
			if !isGoRunTemp(exe) {
				return exe, nil
			}
		}
	}

	// Second: look in GOPATH/bin or GOBIN
	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot determine GOPATH: %w", err)
			}

			gopath = filepath.Join(home, "go")
		}

		gobin = filepath.Join(gopath, "bin")
	}

	name := "repited"
	if runtime.GOOS == "windows" {
		name = "repited.exe"
	}

	candidate := filepath.Join(gobin, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// Third: try PATH
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("repited binary not found — install with: go install github.com/inovacc/repited@latest")
}

// isGoRunTemp returns true if the path looks like a `go run` temp binary.
func isGoRunTemp(path string) bool {
	// go run creates binaries in temp directories
	tmpDir := os.TempDir()
	localTemp := filepath.Join(os.Getenv("LOCALAPPDATA"), "Temp")

	return len(path) > 0 && (strings.HasPrefix(filepath.Clean(path), filepath.Clean(tmpDir)) ||
		strings.HasPrefix(filepath.Clean(path), filepath.Clean(localTemp)))
}
