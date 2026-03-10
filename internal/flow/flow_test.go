package flow

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── NewPipeline ──

func TestNewPipeline(t *testing.T) {
	p := NewPipeline("/tmp/proj", true)
	if p.Dir != "/tmp/proj" {
		t.Errorf("Dir = %q, want %q", p.Dir, "/tmp/proj")
	}

	if !p.Verbose {
		t.Error("Verbose should be true")
	}

	if len(p.Steps) != 0 {
		t.Error("Steps should be empty")
	}

	if len(p.Results) != 0 {
		t.Error("Results should be empty")
	}
}

// ── Add ──

func TestAddSetsDefaults(t *testing.T) {
	p := NewPipeline("/proj", false)
	p.Add(Step{Name: "build", Cmd: "go", Args: []string{"build", "./..."}})

	if len(p.Steps) != 1 {
		t.Fatal("expected 1 step")
	}

	s := p.Steps[0]
	if s.Dir != "/proj" {
		t.Errorf("Dir should inherit pipeline dir, got %q", s.Dir)
	}

	if s.OnFail != "stop" {
		t.Errorf("OnFail should default to 'stop', got %q", s.OnFail)
	}
}

func TestAddPreservesExplicitValues(t *testing.T) {
	p := NewPipeline("/proj", false)
	p.Add(Step{Name: "test", Cmd: "go", Dir: "/other", OnFail: "warn"})

	s := p.Steps[0]
	if s.Dir != "/other" {
		t.Errorf("Dir should be preserved as /other, got %q", s.Dir)
	}

	if s.OnFail != "warn" {
		t.Errorf("OnFail should be preserved as warn, got %q", s.OnFail)
	}
}

// ── Run ──

func echoCmd() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}

	return "echo"
}

func echoArgs(msg string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/c", "echo", msg}
	}

	return []string{msg}
}

func falseCmd() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "exit", "1"}
	}

	return "false", nil
}

func TestRunSuccess(t *testing.T) {
	p := NewPipeline(t.TempDir(), false)
	p.Quiet = true
	p.Add(Step{Name: "greet", Cmd: echoCmd(), Args: echoArgs("hello")})

	if err := p.Run(); err != nil {
		t.Fatal(err)
	}

	if len(p.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(p.Results))
	}

	if p.Results[0].Status != "ok" {
		t.Errorf("status = %q, want ok", p.Results[0].Status)
	}
}

func TestRunSkipped(t *testing.T) {
	p := NewPipeline(t.TempDir(), false)
	p.Quiet = true
	p.Add(Step{Name: "skipped", Cmd: "echo", Skip: true})

	if err := p.Run(); err != nil {
		t.Fatal(err)
	}

	if p.Results[0].Status != "skipped" {
		t.Errorf("status = %q, want skipped", p.Results[0].Status)
	}
}

func TestRunRequireMissing(t *testing.T) {
	p := NewPipeline(t.TempDir(), false)
	p.Quiet = true
	p.Add(Step{Name: "need-file", Cmd: echoCmd(), Args: echoArgs("hi"), Require: "nonexistent.txt"})

	if err := p.Run(); err != nil {
		t.Fatal(err)
	}

	if p.Results[0].Status != "skipped" {
		t.Errorf("status = %q, want skipped (missing requirement)", p.Results[0].Status)
	}
}

func TestRunRequireExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPipeline(dir, false)
	p.Quiet = true
	p.Add(Step{Name: "build", Cmd: echoCmd(), Args: echoArgs("building"), Require: "go.mod"})

	if err := p.Run(); err != nil {
		t.Fatal(err)
	}

	if p.Results[0].Status != "ok" {
		t.Errorf("status = %q, want ok", p.Results[0].Status)
	}
}

func TestRunOnFailStop(t *testing.T) {
	cmd, args := falseCmd()
	p := NewPipeline(t.TempDir(), false)
	p.Quiet = true
	p.Add(Step{Name: "fail", Cmd: cmd, Args: args, OnFail: "stop"})
	p.Add(Step{Name: "after", Cmd: echoCmd(), Args: echoArgs("after")})

	err := p.Run()
	if err == nil {
		t.Fatal("expected error from failing step")
	}
	// Only 1 result (second step not reached)
	if len(p.Results) != 1 {
		t.Errorf("expected 1 result (stopped), got %d", len(p.Results))
	}

	if p.Results[0].Status != "failed" {
		t.Errorf("status = %q, want failed", p.Results[0].Status)
	}
}

func TestRunOnFailWarn(t *testing.T) {
	cmd, args := falseCmd()
	p := NewPipeline(t.TempDir(), false)
	p.Quiet = true
	p.Add(Step{Name: "warn-step", Cmd: cmd, Args: args, OnFail: "warn"})
	p.Add(Step{Name: "continue", Cmd: echoCmd(), Args: echoArgs("ok")})

	err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(p.Results))
	}

	if p.Results[0].Status != "warned" {
		t.Errorf("step 1 status = %q, want warned", p.Results[0].Status)
	}

	if p.Results[1].Status != "ok" {
		t.Errorf("step 2 status = %q, want ok", p.Results[1].Status)
	}
}

func TestRunOnFailSkip(t *testing.T) {
	cmd, args := falseCmd()
	p := NewPipeline(t.TempDir(), false)
	p.Quiet = true
	p.Add(Step{Name: "skip-step", Cmd: cmd, Args: args, OnFail: "skip"})
	p.Add(Step{Name: "continue", Cmd: echoCmd(), Args: echoArgs("ok")})

	err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	if p.Results[0].Status != "skipped" {
		t.Errorf("step 1 status = %q, want skipped", p.Results[0].Status)
	}
}

func TestRunEmpty(t *testing.T) {
	p := NewPipeline(t.TempDir(), false)

	p.Quiet = true
	if err := p.Run(); err != nil {
		t.Fatal(err)
	}

	if len(p.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(p.Results))
	}
}

func TestRunDuration(t *testing.T) {
	p := NewPipeline(t.TempDir(), false)
	p.Quiet = true
	p.Add(Step{Name: "echo", Cmd: echoCmd(), Args: echoArgs("fast")})

	if err := p.Run(); err != nil {
		t.Fatal(err)
	}

	if p.Results[0].Duration <= 0 {
		t.Error("duration should be positive")
	}
}

// ── printIndented ──

func TestPrintIndented(t *testing.T) {
	// Mostly a smoke test — function prints to stdout
	printIndented("") // should return early
	printIndented("single line output")
	// 25 lines → should truncate to last 20
	var long strings.Builder
	for range 25 {
		long.WriteString("line\n")
	}

	printIndented(long.String())
}

// ── Summary ──

func TestSummary(t *testing.T) {
	p := NewPipeline(t.TempDir(), false)
	p.Results = []Result{
		{Status: "ok"},
		{Status: "ok"},
		{Status: "warned"},
		{Status: "skipped"},
		{Status: "failed"},
	}
	// Smoke test — just verify it doesn't panic
	p.Summary()
}

// ── Mixed pipeline ──

func TestMixedPipeline(t *testing.T) {
	cmd, args := falseCmd()
	p := NewPipeline(t.TempDir(), false)
	p.Quiet = true

	p.Add(Step{Name: "ok1", Cmd: echoCmd(), Args: echoArgs("1")})
	p.Add(Step{Name: "skip1", Cmd: "ignored", Skip: true})
	p.Add(Step{Name: "warn1", Cmd: cmd, Args: args, OnFail: "warn"})
	p.Add(Step{Name: "ok2", Cmd: echoCmd(), Args: echoArgs("2")})

	if err := p.Run(); err != nil {
		t.Fatal(err)
	}

	expected := []string{"ok", "skipped", "warned", "ok"}
	if len(p.Results) != len(expected) {
		t.Fatalf("got %d results, want %d", len(p.Results), len(expected))
	}

	for i, want := range expected {
		if p.Results[i].Status != want {
			t.Errorf("result[%d].Status = %q, want %q", i, p.Results[i].Status, want)
		}
	}
}
