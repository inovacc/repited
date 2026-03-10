package deps

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Dep represents an external tool dependency.
type Dep struct {
	Name       string // binary name (e.g., "omni", "scout")
	InstallCmd string // go install path
	Required   bool   // if true, auto-install; if false, just warn
}

// KnownDeps is the list of tools repited depends on.
var KnownDeps = []Dep{
	{Name: "omni", InstallCmd: "github.com/inovacc/omni@latest", Required: true},
	{Name: "scout", InstallCmd: "github.com/inovacc/scout@latest", Required: true},
	{Name: "golangci-lint", InstallCmd: "", Required: false}, // installed separately
}

// EnsureInstalled checks if a tool is available and installs it if missing.
func EnsureInstalled(name string) error {
	if IsInstalled(name) {
		return nil
	}

	for _, dep := range KnownDeps {
		if dep.Name == name {
			if dep.InstallCmd == "" {
				return fmt.Errorf("%s is not installed and has no auto-install path", name)
			}

			return Install(dep)
		}
	}

	return fmt.Errorf("unknown dependency: %s", name)
}

// EnsureAll checks and installs all required dependencies.
func EnsureAll() []string {
	var installed []string

	for _, dep := range KnownDeps {
		if !dep.Required {
			continue
		}

		if IsInstalled(dep.Name) {
			continue
		}

		if dep.InstallCmd == "" {
			continue
		}

		if err := Install(dep); err == nil {
			installed = append(installed, dep.Name)
		}
	}

	return installed
}

// IsInstalled checks if a binary is available in PATH.
func IsInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Install runs go install for a dependency.
func Install(dep Dep) error {
	if dep.InstallCmd == "" {
		return fmt.Errorf("no install command for %s", dep.Name)
	}

	fmt.Fprintf(os.Stderr, "repited: installing %s (%s)...\n", dep.Name, dep.InstallCmd)
	cmd := exec.Command("go", "install", dep.InstallCmd)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing %s: %w", dep.Name, err)
	}

	fmt.Fprintf(os.Stderr, "repited: %s installed successfully\n", dep.Name)

	return nil
}

// Status returns a human-readable status of all dependencies.
func Status() string {
	var sb strings.Builder

	for _, dep := range KnownDeps {
		status := "installed"

		if !IsInstalled(dep.Name) {
			if dep.Required {
				status = "MISSING (auto-installable)"
			} else {
				status = "missing (optional)"
			}
		}

		fmt.Fprintf(&sb, "  %-20s %s\n", dep.Name, status)
	}

	return sb.String()
}
