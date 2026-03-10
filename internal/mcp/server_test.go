package mcpserver

import (
	"testing"
)

// ── textResult ──

func TestTextResult(t *testing.T) {
	r := textResult("hello", false)
	if r.IsError {
		t.Error("should not be error")
	}

	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(r.Content))
	}
}

func TestTextResultError(t *testing.T) {
	r := textResult("fail", true)
	if !r.IsError {
		t.Error("should be error")
	}
}

// ── shouldSkip ──

func TestShouldSkipWithSkipSet(t *testing.T) {
	skip := map[string]bool{"lint": true, "test": true}
	only := map[string]bool{}

	if !shouldSkip("lint", skip, only) {
		t.Error("lint should be skipped")
	}

	if !shouldSkip("test", skip, only) {
		t.Error("test should be skipped")
	}

	if shouldSkip("build", skip, only) {
		t.Error("build should not be skipped")
	}
}

func TestShouldSkipWithOnlySet(t *testing.T) {
	skip := map[string]bool{}
	only := map[string]bool{"build": true, "stage": true}

	if shouldSkip("build", skip, only) {
		t.Error("build should NOT be skipped (in only set)")
	}

	if !shouldSkip("lint", skip, only) {
		t.Error("lint should be skipped (not in only set)")
	}
}

func TestShouldSkipOnlyOverridesSkip(t *testing.T) {
	skip := map[string]bool{"build": true}
	only := map[string]bool{"build": true}

	// When only is non-empty, it takes precedence
	if shouldSkip("build", skip, only) {
		t.Error("build should NOT be skipped (only set takes precedence)")
	}
}

func TestShouldSkipEmpty(t *testing.T) {
	skip := map[string]bool{}
	only := map[string]bool{}

	if shouldSkip("anything", skip, only) {
		t.Error("nothing should be skipped with empty sets")
	}
}

// ── toSet ──

func TestToSet(t *testing.T) {
	set := toSet([]string{"lint", "test"})
	if !set["lint"] {
		t.Error("set should contain lint")
	}

	if !set["test"] {
		t.Error("set should contain test")
	}

	if set["build"] {
		t.Error("set should not contain build")
	}
}

func TestToSetCommaSeparated(t *testing.T) {
	set := toSet([]string{"lint,test", "build"})
	if !set["lint"] {
		t.Error("set should contain lint")
	}

	if !set["test"] {
		t.Error("set should contain test")
	}

	if !set["build"] {
		t.Error("set should contain build")
	}
}

func TestToSetEmpty(t *testing.T) {
	set := toSet(nil)
	if len(set) != 0 {
		t.Errorf("empty input should produce empty set, got %v", set)
	}
}

func TestToSetTrimsSpaces(t *testing.T) {
	set := toSet([]string{" lint , test "})
	if !set["lint"] {
		t.Error("set should contain lint (trimmed)")
	}

	if !set["test"] {
		t.Error("set should contain test (trimmed)")
	}
}

// ── fileExists / dirExists ──

func TestFileExists(t *testing.T) {
	// go.mod exists at the repo root but we're in internal/mcp
	if fileExists("definitely-not-a-file.xyz") {
		t.Error("nonexistent file should return false")
	}
}

func TestDirExists(t *testing.T) {
	if !dirExists(".") {
		t.Error("current dir should exist")
	}

	if dirExists("nonexistent-dir-xyz") {
		t.Error("nonexistent dir should return false")
	}
}

// ── truncate ──

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 100) != short {
		t.Error("short string should not be truncated")
	}

	long := "abcdefghij"

	result := truncate(long, 5)
	if result != "abcde\n... (truncated)" {
		t.Errorf("truncate = %q, want %q", result, "abcde\n... (truncated)")
	}
}

// ── suggestNextSteps ──

func TestSuggestNextStepsPush(t *testing.T) {
	dir := t.TempDir()

	suggestions := suggestNextSteps("push", dir)
	if len(suggestions) == 0 {
		t.Error("expected suggestions for push")
	}
	// First suggestion should be about CI
	if suggestions[0].title == "" {
		t.Error("suggestion should have a title")
	}
}

func TestSuggestNextStepsCommit(t *testing.T) {
	suggestions := suggestNextSteps("commit", t.TempDir())
	if len(suggestions) == 0 {
		t.Error("expected suggestions for commit")
	}
}

func TestSuggestNextStepsMerge(t *testing.T) {
	suggestions := suggestNextSteps("merge", t.TempDir())
	if len(suggestions) == 0 {
		t.Error("expected suggestions for merge")
	}
}

func TestSuggestNextStepsRelease(t *testing.T) {
	suggestions := suggestNextSteps("release", t.TempDir())
	if len(suggestions) == 0 {
		t.Error("expected suggestions for release")
	}
}

func TestSuggestNextStepsDeploy(t *testing.T) {
	suggestions := suggestNextSteps("deploy", t.TempDir())
	if len(suggestions) == 0 {
		t.Error("expected suggestions for deploy")
	}
}

func TestSuggestNextStepsTest(t *testing.T) {
	suggestions := suggestNextSteps("test", t.TempDir())
	if len(suggestions) == 0 {
		t.Error("expected suggestions for test")
	}
}

func TestSuggestNextStepsRefactor(t *testing.T) {
	suggestions := suggestNextSteps("refactor", t.TempDir())
	if len(suggestions) == 0 {
		t.Error("expected suggestions for refactor")
	}
}

func TestSuggestNextStepsUnknown(t *testing.T) {
	suggestions := suggestNextSteps("unknown-action", t.TempDir())
	if len(suggestions) == 0 {
		t.Error("expected default suggestions for unknown action")
	}
}
