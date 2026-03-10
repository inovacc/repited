package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// ── isShellSyntax ──

func TestIsShellSyntax(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"if [ -f foo ]; then", true},
		{"then", true},
		{"else", true},
		{"elif x", true},
		{"fi", true},
		{"for i in *; do", true},
		{"do", true},
		{"done", true},
		{"while true; do", true},
		{"until false; do", true},
		{"case $x in", true},
		{"esac", true},
		{"select opt in *", true},
		{"function foo", true},
		{"}", true},
		{"{", true},
		{"[[", true},
		{"]]", true},
		{"((", true},
		{"))", true},
		// Not shell syntax
		{"go build ./...", false},
		{"git commit -m msg", false},
		{"echo hello", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := isShellSyntax(tt.line); got != tt.want {
				t.Errorf("isShellSyntax(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// ── isCodeFragment ──

func TestIsCodeFragment(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"EOF", true},
		{"EOF'", true},
		{"'EOF'", true},
		{"-flag value", true},
		{") something", true},
		{"] done", true},
		{`"quoted"`, true},
		{"'single'", true},
		{"(group)", true},
		{"{block}", true},
		{"> redirect", true},
		{"< input", true},
		{"& background", true},
		{"| pipe", true},
		{"func main()", true},
		{"type Foo struct", true},
		{"package main", true},
		{"def some_func():", true},
		{"class MyClass:", true},
		{"fmt.Println()", true},
		{"log.Fatal()", true},
		{"os.Exit(1)", true},
		// Not code
		{"go build ./...", false},
		{"git status", false},
		{"docker run -d nginx", false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := isCodeFragment(tt.line); got != tt.want {
				t.Errorf("isCodeFragment(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// ── splitStatements ──

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{"simple", "go build", []string{"go build"}},
		{"and chain", "go build && go test", []string{"go build", "go test"}},
		{"or chain", "go build || echo fail", []string{"go build", "echo fail"}},
		{"semicolons", "go build; go test", []string{"go build", "go test"}},
		{"pipe", "go test | grep PASS", []string{"go test", "grep PASS"}},
		{"mixed", "go mod tidy && go build || exit 1; echo done", []string{"go mod tidy", "go build", "exit 1", "echo done"}},
		{"empty parts", "  &&  || ;  ", nil},
		{"triple chain", "a && b && c", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitStatements(tt.line)
			if len(got) != len(tt.want) {
				t.Fatalf("splitStatements(%q) = %v (len %d), want %v (len %d)", tt.line, got, len(got), tt.want, len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitStatements(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ── extractTool ──

func TestExtractTool(t *testing.T) {
	tests := []struct {
		stmt string
		want string
	}{
		// Multi-word tools
		{"go build ./...", "go build"},
		{"git commit -m 'msg'", "git commit"},
		{"gh pr create", "gh pr"},
		{"docker build -t img .", "docker build"},
		{"kubectl apply -f deploy.yaml", "kubectl apply"},
		{"npm install", "npm install"},
		{"cargo test", "cargo test"},
		{"terraform plan", "terraform plan"},

		// Omni
		{"omni cat file.go", "omni cat"},

		// Variable assignment prefix
		{"GOOS=linux go build", "go build"},
		{"FOO=bar BAZ=1 mycommand", "mycommand"},

		// Shell builtins (should be skipped)
		{"cd /tmp", ""},
		{"export FOO=bar", ""},
		{"echo hello", ""},
		{"exit 1", ""},

		// Invalid commands
		{"ALLCAPS", ""},
		{"/usr/bin/foo", ""},
		{"file.go", ""},
		{"--flag", ""},
		{"", ""},

		// Single-word valid commands
		{"golangci-lint run --fix", "golangci-lint"},
		{"myapp --verbose", "myapp"},

		// Noise words
		{"do something", ""},
		{"with args", ""},

		// Multi-word with flags
		{"go test -v -count=1 ./...", "go test"},
		{"git add -A", "git add"},

		// Tool with flag as second arg (not multi-word since flag starts with -)
		{"go -version", "go"},
	}
	for _, tt := range tests {
		t.Run(tt.stmt, func(t *testing.T) {
			if got := extractTool(tt.stmt); got != tt.want {
				t.Errorf("extractTool(%q) = %q, want %q", tt.stmt, got, tt.want)
			}
		})
	}
}

// ── isValidCommand ──

func TestIsValidCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"golangci-lint", true},
		{"myapp", true},
		{"my-tool", true},
		{"my_tool", true},
		{"my.tool", true},

		// Invalid
		{"", false},
		{"Capitalized", false},
		{"ALL_UPPER", false},
		{"/path/to/cmd", false},
		{"file.go", false},
		{"file.md", false},
		{"file.yml", false},
		{"file.json", false},
		{"file.sh", false},
		{"file.py", false},
		{"do", false},          // noise
		{"with", false},        // noise
		{"expected", false},    // noise
		{"cmd@version", false}, // invalid char @
		{"bg-red-500", false},  // CSS class
		{"px-4", false},        // CSS class
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := isValidCommand(tt.cmd); got != tt.want {
				t.Errorf("isValidCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

// ── isDir ──

func TestIsDir(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !isDir(dir) {
		t.Error("isDir should return true for a directory")
	}

	if isDir(file) {
		t.Error("isDir should return false for a regular file")
	}

	if isDir(filepath.Join(dir, "nonexistent")) {
		t.Error("isDir should return false for nonexistent path")
	}
}

// ── extractCommands ──

func TestExtractCommands(t *testing.T) {
	dir := t.TempDir()

	// Write a sample script
	script := filepath.Join(dir, "build.sh")

	content := `#!/bin/bash
# Build script
go mod tidy && go build ./...
go vet ./...
go test -count=1 ./...
# Stage and commit
git add .
git commit -m "build"
`
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractCommands(script)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"go mod", "go build", "go vet", "go test", "git add", "git commit"}
	if len(cmds) != len(expected) {
		t.Fatalf("extractCommands got %d commands %v, want %d %v", len(cmds), cmds, len(expected), expected)
	}

	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestExtractCommandsSkipsNonShell(t *testing.T) {
	dir := t.TempDir()

	pyFile := filepath.Join(dir, "script.py")
	if err := os.WriteFile(pyFile, []byte("print('hello')"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractCommands(pyFile)
	if err != nil {
		t.Fatal(err)
	}

	if cmds != nil {
		t.Errorf("expected nil for .py file, got %v", cmds)
	}
}

func TestExtractCommandsCodeFragments(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "mixed.sh")

	content := `#!/bin/bash
go build ./...
func main() {
type Foo struct {
fmt.Println("hello")
git status
`
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds, err := extractCommands(script)
	if err != nil {
		t.Fatal(err)
	}

	// Should only get "go build" and "git status", not the code fragments
	if len(cmds) != 2 {
		t.Fatalf("got %d commands %v, want 2", len(cmds), cmds)
	}

	if cmds[0] != "go build" {
		t.Errorf("command[0] = %q, want %q", cmds[0], "go build")
	}

	if cmds[1] != "git status" {
		t.Errorf("command[1] = %q, want %q", cmds[1], "git status")
	}
}

// ── readScripts ──

func TestReadScripts(t *testing.T) {
	dir := t.TempDir()

	// Create a script
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte("go build ./...\ngit status\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory (should be skipped)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	scripts, err := readScripts(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(scripts) != 1 {
		t.Fatalf("got %d scripts, want 1", len(scripts))
	}

	if scripts[0].Name != "run.sh" {
		t.Errorf("script name = %q, want %q", scripts[0].Name, "run.sh")
	}

	if len(scripts[0].Commands) != 2 {
		t.Errorf("got %d commands, want 2", len(scripts[0].Commands))
	}
}

// ── Scan (integration) ──

func TestScanFindsProject(t *testing.T) {
	root := t.TempDir()

	// Create a project with .git and .scripts
	proj := filepath.Join(root, "myproject")
	if err := os.MkdirAll(filepath.Join(proj, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptsDir := filepath.Join(proj, ".scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(scriptsDir, "build.sh"), []byte("go build ./...\ngo test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(root, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Projects) != 1 {
		t.Fatalf("found %d projects, want 1", len(result.Projects))
	}

	if result.Projects[0].Path != proj {
		t.Errorf("project path = %q, want %q", result.Projects[0].Path, proj)
	}

	if len(result.ToolCounts) == 0 {
		t.Error("expected tool counts, got none")
	}
}

func TestScanEmptyDir(t *testing.T) {
	root := t.TempDir()

	result, err := Scan(root, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(result.Projects))
	}
}

func TestScanSkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()

	// Hidden dir with .git + .scripts inside (should be skipped)
	hidden := filepath.Join(root, ".hidden", "proj")
	if err := os.MkdirAll(filepath.Join(hidden, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(hidden, ".scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(hidden, ".scripts", "x.sh"), []byte("go build\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Projects) != 0 {
		t.Errorf("expected 0 projects (hidden skipped), got %d", len(result.Projects))
	}
}

func TestScanDepthLimit(t *testing.T) {
	root := t.TempDir()

	// Project at depth 4 (too deep for maxDepth=2)
	deep := filepath.Join(root, "a", "b", "c", "d")
	if err := os.MkdirAll(filepath.Join(deep, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(deep, ".scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(deep, ".scripts", "x.sh"), []byte("go build\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(root, 2)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Projects) != 0 {
		t.Errorf("expected 0 projects (depth exceeded), got %d", len(result.Projects))
	}
}

func TestScanToolCountsSorted(t *testing.T) {
	root := t.TempDir()

	proj := filepath.Join(root, "proj")
	if err := os.MkdirAll(filepath.Join(proj, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptsDir := filepath.Join(proj, ".scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// git status appears 3 times, go build once
	script := "git status\ngit status\ngit status\ngo build ./...\n"
	if err := os.WriteFile(filepath.Join(scriptsDir, "run.sh"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(root, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.ToolCounts) < 2 {
		t.Fatalf("expected >= 2 tool counts, got %d", len(result.ToolCounts))
	}
	// First should be the most frequent
	if result.ToolCounts[0].Count < result.ToolCounts[1].Count {
		t.Error("tool counts should be sorted descending")
	}
}
