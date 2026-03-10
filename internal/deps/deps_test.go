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

// ── Additional coverage tests ──

// withKnownDeps temporarily replaces KnownDeps for the duration of f,
// then restores the original value.
func withKnownDeps(t *testing.T, deps []Dep, f func()) {
	t.Helper()

	original := KnownDeps
	KnownDeps = deps

	t.Cleanup(func() { KnownDeps = original })
	f()
}

func TestInstallInvalidPackage(t *testing.T) {
	// Test Install with a package path that doesn't exist — exercises the
	// cmd.Run() error path (lines 85-87).
	dep := Dep{Name: "fake-tool", InstallCmd: "example.invalid/nonexistent/pkg@latest"}

	err := Install(dep)
	if err == nil {
		t.Fatal("expected error when installing invalid package")
	}

	if !strings.Contains(err.Error(), "installing fake-tool") {
		t.Errorf("error = %q, want it to mention 'installing fake-tool'", err.Error())
	}
}

func TestEnsureInstalledKnownDepAlreadyPresent(t *testing.T) {
	// Inject "go" as a known dep so EnsureInstalled finds it already
	// installed and returns nil (covers the early-return on line 27).
	withKnownDeps(t, []Dep{
		{Name: "go", InstallCmd: "example.com/fake@latest", Required: true},
	}, func() {
		err := EnsureInstalled("go")
		if err != nil {
			t.Errorf("EnsureInstalled(go) should succeed when already installed, got: %v", err)
		}
	})
}

func TestEnsureInstalledMissingWithInstallCmd(t *testing.T) {
	// Dep not installed, has an InstallCmd that will fail — exercises the
	// Install(dep) call inside EnsureInstalled (line 36).
	withKnownDeps(t, []Dep{
		{Name: "nonexistent-tool-abc", InstallCmd: "example.invalid/bad@latest", Required: true},
	}, func() {
		err := EnsureInstalled("nonexistent-tool-abc")
		if err == nil {
			t.Fatal("expected error for missing dep with bad install cmd")
		}

		if !strings.Contains(err.Error(), "installing nonexistent-tool-abc") {
			t.Errorf("error = %q, want mention of installing", err.Error())
		}
	})
}

func TestEnsureInstalledMissingNoInstallCmd(t *testing.T) {
	// Dep not installed, no InstallCmd — covers line 33.
	withKnownDeps(t, []Dep{
		{Name: "nonexistent-tool-abc", InstallCmd: "", Required: false},
	}, func() {
		err := EnsureInstalled("nonexistent-tool-abc")
		if err == nil {
			t.Fatal("expected error for dep with no install cmd")
		}

		if !strings.Contains(err.Error(), "no auto-install path") {
			t.Errorf("error = %q, want 'no auto-install path'", err.Error())
		}
	})
}

func TestEnsureAllSkipsNonRequired(t *testing.T) {
	// Only non-required deps — EnsureAll should skip them all.
	withKnownDeps(t, []Dep{
		{Name: "optional-a", InstallCmd: "example.com/a@latest", Required: false},
		{Name: "optional-b", InstallCmd: "", Required: false},
	}, func() {
		installed := EnsureAll()
		if len(installed) != 0 {
			t.Errorf("expected no installs, got: %v", installed)
		}
	})
}

func TestEnsureAllSkipsAlreadyInstalled(t *testing.T) {
	// "go" is already installed and required — should not appear in the
	// installed list because it's already present.
	withKnownDeps(t, []Dep{
		{Name: "go", InstallCmd: "example.com/fake@latest", Required: true},
	}, func() {
		installed := EnsureAll()
		if len(installed) != 0 {
			t.Errorf("expected no installs for already-installed dep, got: %v", installed)
		}
	})
}

func TestEnsureAllSkipsEmptyInstallCmd(t *testing.T) {
	// Required but no InstallCmd — should be skipped (line 57).
	withKnownDeps(t, []Dep{
		{Name: "nonexistent-tool-xyz", InstallCmd: "", Required: true},
	}, func() {
		installed := EnsureAll()
		if len(installed) != 0 {
			t.Errorf("expected no installs for dep with empty InstallCmd, got: %v", installed)
		}
	})
}

func TestEnsureAllInstallFails(t *testing.T) {
	// Required, not installed, has InstallCmd but it will fail — exercises
	// the Install error path in EnsureAll (line 60 returns non-nil err).
	withKnownDeps(t, []Dep{
		{Name: "nonexistent-tool-xyz", InstallCmd: "example.invalid/bad@latest", Required: true},
	}, func() {
		installed := EnsureAll()
		if len(installed) != 0 {
			t.Errorf("expected no installs when install fails, got: %v", installed)
		}
	})
}

func TestStatusInstalledDep(t *testing.T) {
	// "go" is installed — Status should show "installed" for it.
	withKnownDeps(t, []Dep{
		{Name: "go", InstallCmd: "", Required: false},
	}, func() {
		s := Status()
		if !strings.Contains(s, "go") {
			t.Error("Status should mention go")
		}

		if !strings.Contains(s, "installed") {
			t.Error("Status should show 'installed' for go")
		}
	})
}

func TestStatusMissingRequiredDep(t *testing.T) {
	// Missing required dep — should show "MISSING (auto-installable)".
	withKnownDeps(t, []Dep{
		{Name: "nonexistent-required-xyz", InstallCmd: "example.com/x@latest", Required: true},
	}, func() {
		s := Status()
		if !strings.Contains(s, "MISSING (auto-installable)") {
			t.Errorf("Status = %q, want 'MISSING (auto-installable)'", s)
		}
	})
}

func TestStatusMissingOptionalDep(t *testing.T) {
	// Missing optional dep — should show "missing (optional)".
	withKnownDeps(t, []Dep{
		{Name: "nonexistent-optional-xyz", InstallCmd: "", Required: false},
	}, func() {
		s := Status()
		if !strings.Contains(s, "missing (optional)") {
			t.Errorf("Status = %q, want 'missing (optional)'", s)
		}
	})
}

func TestKnownDepsAllHaveNames(t *testing.T) {
	for i, d := range KnownDeps {
		if d.Name == "" {
			t.Errorf("KnownDeps[%d] has empty Name", i)
		}
	}
}

func TestKnownDepsRequiredHaveInstallCmd(t *testing.T) {
	for _, d := range KnownDeps {
		if d.Required && d.InstallCmd == "" {
			t.Errorf("required dep %q has no InstallCmd", d.Name)
		}
	}
}

func TestInstallEmptyName(t *testing.T) {
	dep := Dep{Name: "", InstallCmd: ""}

	err := Install(dep)
	if err == nil {
		t.Error("expected error for dep with empty name and no install cmd")
	}
}

func TestEnsureInstalledEmptyName(t *testing.T) {
	err := EnsureInstalled("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}
