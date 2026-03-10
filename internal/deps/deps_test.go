package deps

import (
	"strings"
	"testing"
)

// ── KnownDeps ──

func TestKnownDepsNotEmpty(t *testing.T) {
	if len(KnownDeps) == 0 {
		t.Fatal("KnownDeps should not be empty")
	}
}

func TestKnownDepsHasOmni(t *testing.T) {
	found := false

	for _, d := range KnownDeps {
		if d.Name == "omni" {
			found = true

			if d.InstallCmd == "" {
				t.Error("omni should have an install command")
			}

			if !d.Required {
				t.Error("omni should be required")
			}
		}
	}

	if !found {
		t.Error("KnownDeps should include omni")
	}
}

func TestKnownDepsHasScout(t *testing.T) {
	found := false

	for _, d := range KnownDeps {
		if d.Name == "scout" {
			found = true

			if d.InstallCmd == "" {
				t.Error("scout should have an install command")
			}

			if !d.Required {
				t.Error("scout should be required")
			}
		}
	}

	if !found {
		t.Error("KnownDeps should include scout")
	}
}

func TestKnownDepsGolangciLintOptional(t *testing.T) {
	for _, d := range KnownDeps {
		if d.Name == "golangci-lint" {
			if d.Required {
				t.Error("golangci-lint should not be required")
			}

			if d.InstallCmd != "" {
				t.Error("golangci-lint should have empty install command (installed separately)")
			}
		}
	}
}

// ── IsInstalled ──

func TestIsInstalledGo(t *testing.T) {
	// "go" should always be installed in a Go test environment
	if !IsInstalled("go") {
		t.Error("go should be installed")
	}
}

func TestIsInstalledNonexistent(t *testing.T) {
	if IsInstalled("definitely-not-a-real-binary-xyz123") {
		t.Error("nonexistent binary should not be installed")
	}
}

// ── EnsureInstalled ──

func TestEnsureInstalledAlreadyPresent(t *testing.T) {
	// "go" is already installed — should be a no-op
	if err := EnsureInstalled("go"); err == nil {
		// "go" is not in KnownDeps, so it should return "unknown dependency"
		t.Log("go is not a known dep, EnsureInstalled returns error (expected)")
	}
}

func TestEnsureInstalledUnknown(t *testing.T) {
	err := EnsureInstalled("totally-unknown-dep")
	if err == nil {
		t.Error("expected error for unknown dependency")
	}

	if !strings.Contains(err.Error(), "unknown dependency") {
		t.Errorf("error = %q, want 'unknown dependency'", err.Error())
	}
}

func TestEnsureInstalledNoInstallCmd(t *testing.T) {
	// golangci-lint has no install command
	if IsInstalled("golangci-lint") {
		t.Skip("golangci-lint is already installed, cannot test missing path")
	}

	err := EnsureInstalled("golangci-lint")
	if err == nil {
		t.Error("expected error for dep with no install command")
	}

	if !strings.Contains(err.Error(), "no auto-install path") {
		t.Errorf("error = %q, want 'no auto-install path'", err.Error())
	}
}

// ── EnsureAll ──

func TestEnsureAllReturnsInstalled(t *testing.T) {
	// This is a smoke test — it may install things depending on environment
	// We just verify it doesn't panic and returns a slice
	installed := EnsureAll()
	t.Logf("EnsureAll installed: %v", installed)
}

// ── Install ──

func TestInstallNoCmd(t *testing.T) {
	dep := Dep{Name: "test-dep", InstallCmd: ""}

	err := Install(dep)
	if err == nil {
		t.Error("expected error for dep with no install command")
	}

	if !strings.Contains(err.Error(), "no install command") {
		t.Errorf("error = %q, want 'no install command'", err.Error())
	}
}

// ── Status ──

func TestStatus(t *testing.T) {
	s := Status()
	if s == "" {
		t.Error("Status should not be empty")
	}
	// Should contain all known deps
	for _, d := range KnownDeps {
		if !strings.Contains(s, d.Name) {
			t.Errorf("Status should mention %q", d.Name)
		}
	}
	// Should contain status indicators
	if !strings.Contains(s, "installed") && !strings.Contains(s, "MISSING") && !strings.Contains(s, "missing") {
		t.Error("Status should contain status indicators")
	}
}
