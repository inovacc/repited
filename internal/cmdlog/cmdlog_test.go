package cmdlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── DataDir ──

func TestDataDir(t *testing.T) {
	dir := DataDir()
	if dir == "" {
		t.Error("DataDir should not be empty")
	}

	if !strings.HasSuffix(dir, "Repited") {
		t.Errorf("DataDir = %q, want suffix 'Repited'", dir)
	}
}

func TestDataDirFallback(t *testing.T) {
	// When LOCALAPPDATA is empty, should still return something reasonable
	orig := os.Getenv("LOCALAPPDATA")

	t.Setenv("LOCALAPPDATA", "")

	defer func() { _ = os.Setenv("LOCALAPPDATA", orig) }()

	dir := DataDir()
	if dir == "" {
		t.Error("DataDir should not be empty even without LOCALAPPDATA")
	}

	if !strings.HasSuffix(dir, "Repited") {
		t.Errorf("DataDir = %q, want suffix 'Repited'", dir)
	}
}

// ── CommandsDir ──

func TestCommandsDir(t *testing.T) {
	dir := CommandsDir()
	if !strings.HasSuffix(dir, filepath.Join("Repited", "commands")) {
		t.Errorf("CommandsDir = %q, want suffix 'Repited/commands'", dir)
	}
}

// ── DBPath ──

func TestDBPath(t *testing.T) {
	path := DBPath()
	if !strings.HasSuffix(path, "repited.db") {
		t.Errorf("DBPath = %q, want suffix 'repited.db'", path)
	}
}

// ── New ──

func TestNew(t *testing.T) {
	log := New("flow", "/proj")
	if log.Command != "flow" {
		t.Errorf("Command = %q, want flow", log.Command)
	}

	if log.Dir != "/proj" {
		t.Errorf("Dir = %q, want /proj", log.Dir)
	}

	if log.ID.IsNil() {
		t.Error("ID should not be nil")
	}

	if len(log.Entries) != 0 {
		t.Error("Entries should be empty")
	}
}

// ── Add ──

func TestAdd(t *testing.T) {
	log := New("scan", "/proj")
	log.Add(Entry{Cmd: "go", Args: []string{"build", "./..."}, Status: "ok", Duration: time.Second})
	log.Add(Entry{Cmd: "go", Args: []string{"test", "./..."}, Status: "warned", Duration: 2 * time.Second})

	if len(log.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(log.Entries))
	}

	if log.Entries[0].Cmd != "go" {
		t.Errorf("entry 0 cmd = %q, want go", log.Entries[0].Cmd)
	}
}

// ── Format ──

func TestFormat(t *testing.T) {
	log := New("flow", "/my/project")
	log.Add(Entry{Cmd: "go", Args: []string{"build", "./..."}, Status: "ok", Duration: 800 * time.Millisecond})
	log.Add(Entry{Cmd: "golangci-lint", Args: []string{"run", "./..."}, Status: "skipped"})
	log.Add(Entry{Cmd: "git", Args: []string{"commit", "-m", "feat: test"}, Status: "failed", Duration: 200 * time.Millisecond})

	content := log.Format()

	if !strings.Contains(content, "# repited flow") {
		t.Error("format should contain command header")
	}

	if !strings.Contains(content, "# dir: /my/project") {
		t.Error("format should contain dir")
	}

	if !strings.Contains(content, "# id:") {
		t.Error("format should contain KSUID")
	}

	if !strings.Contains(content, "[ok 0.8s] go build ./...") {
		t.Error("format should contain ok entry")
	}

	if !strings.Contains(content, "[skip] golangci-lint run ./...") {
		t.Error("format should contain skip entry")
	}

	if !strings.Contains(content, "[failed 0.2s] git commit -m") {
		t.Error("format should contain failed entry")
	}
}

func TestFormatQuotesSpaces(t *testing.T) {
	log := New("flow", "/proj")
	log.Add(Entry{Cmd: "git", Args: []string{"commit", "-m", "feat: add new feature"}, Status: "ok", Duration: time.Second})

	content := log.Format()
	// "feat: add new feature" should be quoted because it has spaces
	if !strings.Contains(content, `"feat: add new feature"`) {
		t.Errorf("args with spaces should be quoted in format, got: %s", content)
	}
}

// ── Save ──

func TestSave(t *testing.T) {
	// Override LOCALAPPDATA to a temp dir so we don't pollute real data
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	log := New("flow", "/my/project")
	log.Add(Entry{Cmd: "go", Args: []string{"build"}, Status: "ok", Duration: time.Second})

	path, err := log.Save()
	if err != nil {
		t.Fatal(err)
	}

	if path == "" {
		t.Error("Save should return the log file path")
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file should exist at %s: %v", path, err)
	}

	// Verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "# repited flow") {
		t.Error("saved file should contain log header")
	}

	// Verify filename format: KSUID_command.txt
	base := filepath.Base(path)
	if !strings.HasSuffix(base, "_flow.txt") {
		t.Errorf("filename = %q, want suffix '_flow.txt'", base)
	}
}

// ── shelljoin ──

func TestShelljoin(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"empty", nil, ""},
		{"simple", []string{"build", "./..."}, "build ./..."},
		{"with spaces", []string{"-m", "feat: add feature"}, `-m "feat: add feature"`},
		{"with quotes", []string{"-m", `it's "great"`}, `-m "it's \"great\""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shelljoin(tt.args)
			if got != tt.want {
				t.Errorf("shelljoin(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
