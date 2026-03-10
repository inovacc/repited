package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ── Makefile: extractMakefileCommands ──

func TestExtractMakefileCommands(t *testing.T) {
	dir := t.TempDir()
	makefile := filepath.Join(dir, "Makefile")

	content := ".PHONY: build test deploy\n\nbuild:\n\tgo build -o myapp ./cmd/myapp\n\tdocker build -t myapp .\n\ntest:\n\tgo test -v ./...\n\ndeploy:\n\tgit tag v1.0.0\n\tdocker push myapp:latest\n"

	if err := os.WriteFile(makefile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractMakefileCommands(makefile)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"go build", "docker build", "go test", "git tag", "docker push"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractMakefileContinuation(t *testing.T) {
	dir := t.TempDir()
	makefile := filepath.Join(dir, "Makefile")

	content := "build:\n\tdocker build \\\n\t\t-t myapp \\\n\t\t.\n\tgo build \\\n\t\t-o myapp ./cmd/myapp\n"

	if err := os.WriteFile(makefile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractMakefileCommands(makefile)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"docker build", "go build"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractMakefileSkipsDirectives(t *testing.T) {
	dir := t.TempDir()
	makefile := filepath.Join(dir, "Makefile")

	content := ".PHONY: all build test\nGO := go\nBINARY ?= myapp\nLDFLAGS += -s -w\ninclude config.mk\n\nifdef DEBUG\nGOFLAGS = -race\nendif\n\nifeq ($(OS),Windows)\nEXT = .exe\nendif\n\nifneq ($(CI),)\nGOFLAGS += -count=1\nendif\n\nall: build test\n\nbuild:\n\tgo build -o $(BINARY) ./...\n\ntest:\n\tgo test ./...\n"

	if err := os.WriteFile(makefile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractMakefileCommands(makefile)
	if err != nil {
		t.Fatal(err)
	}

	// Only the recipe commands should appear, not directives or variable assignments
	expected := []string{"go build", "go test"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractMakefileMakeSubstitution(t *testing.T) {
	dir := t.TempDir()
	makefile := filepath.Join(dir, "Makefile")

	content := "all:\n\t$(MAKE) build\n\t${MAKE} test\n\nbuild:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n"

	if err := os.WriteFile(makefile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractMakefileCommands(makefile)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"make build", "make test", "go build", "go test"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractMakefileSilentAndIgnoreError(t *testing.T) {
	dir := t.TempDir()
	makefile := filepath.Join(dir, "Makefile")

	content := "build:\n\t@go build ./...\n\t-go test ./...\n\t@-docker build -t app .\n"

	if err := os.WriteFile(makefile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractMakefileCommands(makefile)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"go build", "go test", "docker build"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

// ── justfile: extractJustfileCommands ──

func TestExtractJustfileCommands(t *testing.T) {
	dir := t.TempDir()
	justfile := filepath.Join(dir, "justfile")

	content := `# Build recipes
set shell := ["bash", "-c"]

build:
    go build -o myapp ./cmd/myapp
    docker build -t myapp .

test:
    go test -v ./...

deploy: build test
    git tag v1.0.0
    kubectl apply -f deploy.yaml
`

	if err := os.WriteFile(justfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractJustfileCommands(justfile)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"go build", "docker build", "go test", "git tag", "kubectl apply"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractJustfileAtPrefix(t *testing.T) {
	dir := t.TempDir()
	justfile := filepath.Join(dir, "justfile")

	content := `build:
    @go build ./...
    @docker build -t app .
    go test ./...
`

	if err := os.WriteFile(justfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractJustfileCommands(justfile)
	if err != nil {
		t.Fatal(err)
	}

	// @ prefix should be stripped
	expected := []string{"go build", "docker build", "go test"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractJustfileSkipsDirectives(t *testing.T) {
	dir := t.TempDir()
	justfile := filepath.Join(dir, "justfile")

	content := `set shell := ["bash", "-c"]
alias b := build
import "other.just"
export PATH := "/usr/local/bin"

build:
    go build ./...
`

	if err := os.WriteFile(justfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractJustfileCommands(justfile)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"go build"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractJustfileContinuation(t *testing.T) {
	dir := t.TempDir()
	justfile := filepath.Join(dir, "justfile")

	content := `build:
    docker build \
        -t myapp .
    go test \
        -v ./...
`

	if err := os.WriteFile(justfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractJustfileCommands(justfile)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"docker build", "go test"}
	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

// ── Taskfile.yml: extractTaskfileCommands ──

func TestExtractTaskfileCommands(t *testing.T) {
	dir := t.TempDir()
	taskfile := filepath.Join(dir, "Taskfile.yml")

	content := `version: '3'

tasks:
  build:
    cmds:
      - go build -o myapp ./cmd/myapp
      - docker build -t myapp .

  test:
    cmds:
      - go test -v ./...

  deploy:
    cmds:
      - git tag v1.0.0
      - kubectl apply -f deploy.yaml
`

	if err := os.WriteFile(taskfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractTaskfileCommands(taskfile)
	if err != nil {
		t.Fatal(err)
	}

	// Task iteration order is not guaranteed, so sort
	sort.Strings(cmds)

	expected := []string{"docker build", "git tag", "go build", "go test", "kubectl apply"}
	sort.Strings(expected)

	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractTaskfileObjectCmds(t *testing.T) {
	dir := t.TempDir()
	taskfile := filepath.Join(dir, "Taskfile.yml")

	content := `version: '3'

tasks:
  build:
    cmds:
      - cmd: go build -o myapp ./...
      - cmd: docker build -t myapp .
      - task: test

  test:
    cmds:
      - go test ./...
`

	if err := os.WriteFile(taskfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractTaskfileCommands(taskfile)
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(cmds)

	expected := []string{"docker build", "go build", "go test", "task test"}
	sort.Strings(expected)

	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractTaskfileMultilineCmd(t *testing.T) {
	dir := t.TempDir()
	taskfile := filepath.Join(dir, "Taskfile.yml")

	content := `version: '3'

tasks:
  check:
    cmds:
      - go vet ./... && golangci-lint run ./...
      - go test -count=1 ./...
`

	if err := os.WriteFile(taskfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractTaskfileCommands(taskfile)
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(cmds)

	expected := []string{"go test", "go vet", "golangci-lint"}
	sort.Strings(expected)

	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

// ── Scanner integration: task runner files detected during scan ──

func TestScanFindsTaskRunnerFiles(t *testing.T) {
	root := t.TempDir()

	proj := filepath.Join(root, "myproject")

	if err := os.MkdirAll(filepath.Join(proj, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptsDir := filepath.Join(proj, ".scripts")

	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(scriptsDir, "build.sh"), []byte("go build ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Also add a Makefile in the project root
	makeContent := "test:\n\tgo test ./...\n" //nolint:goconst

	if err := os.WriteFile(filepath.Join(proj, "Makefile"), []byte(makeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(root, ScanOptions{MaxDepth: 3})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Projects) != 1 {
		t.Fatalf("found %d projects, want 1", len(result.Projects))
	}

	// Should have scripts from .scripts/ AND from Makefile
	if len(result.Projects[0].Scripts) < 2 {
		t.Errorf("expected >= 2 scripts (shell + Makefile), got %d: %v",
			len(result.Projects[0].Scripts), scriptNames(result.Projects[0].Scripts))
	}
}

func TestScanFindsProjectWithOnlyTaskRunner(t *testing.T) {
	root := t.TempDir()

	proj := filepath.Join(root, "taskproject")

	if err := os.MkdirAll(filepath.Join(proj, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// No .scripts/ directory — only a Taskfile.yml
	taskfileContent := "version: '3'\ntasks:\n  build:\n    cmds:\n      - go build ./...\n"

	if err := os.WriteFile(filepath.Join(proj, "Taskfile.yml"), []byte(taskfileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(root, ScanOptions{MaxDepth: 3})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Projects) != 1 {
		t.Fatalf("found %d projects, want 1", len(result.Projects))
	}

	if len(result.Projects[0].Scripts) != 1 {
		t.Fatalf("found %d scripts, want 1", len(result.Projects[0].Scripts))
	}

	if !strings.EqualFold(result.Projects[0].Scripts[0].Name, "Taskfile.yml") {
		t.Errorf("script name = %q, want case-insensitive match for %q", result.Projects[0].Scripts[0].Name, "Taskfile.yml")
	}
}

// scriptNames returns a list of script names for test output.
func scriptNames(scripts []Script) []string {
	names := make([]string, len(scripts))
	for i, s := range scripts {
		names[i] = s.Name
	}

	return names
}
