package scanner

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Project represents a discovered project with .git and .scripts directories.
type Project struct {
	Path    string
	Scripts []Script
}

// Script represents a single script file inside a .scripts directory.
type Script struct {
	Name     string
	Path     string
	Commands []string
}

// ToolCount tracks how many times a tool/command was called.
type ToolCount struct {
	Name  string
	Count int
}

// ScanResult holds the full scan output.
type ScanResult struct {
	Projects   []Project
	ToolCounts []ToolCount
}

// Scan walks rootDir looking for directories containing both .git and .scripts.
func Scan(rootDir string, maxDepth int) (*ScanResult, error) {
	var projects []Project

	toolFreq := make(map[string]int)

	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolving root dir: %w", err)
	}

	rootDepth := strings.Count(filepath.ToSlash(rootDir), "/")

	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries
		}

		if !d.IsDir() {
			return nil
		}

		// Skip hidden dirs (except the ones we care about)
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != ".scripts" && name != ".git" {
			return filepath.SkipDir
		}

		// Enforce max depth
		if maxDepth > 0 {
			currentDepth := strings.Count(filepath.ToSlash(path), "/") - rootDepth
			if currentDepth > maxDepth {
				return filepath.SkipDir
			}
		}

		// Check if this directory has both .git and .scripts
		gitDir := filepath.Join(path, ".git")
		scriptsDir := filepath.Join(path, ".scripts")

		if !isDir(gitDir) || !isDir(scriptsDir) {
			return nil
		}

		// Found a project — read its scripts
		scripts, err := readScripts(scriptsDir)
		if err != nil {
			return nil //nolint:nilerr // skip unreadable dirs
		}

		for _, s := range scripts {
			for _, cmd := range s.Commands {
				toolFreq[cmd]++
			}
		}

		projects = append(projects, Project{
			Path:    path,
			Scripts: scripts,
		})

		// Don't descend into this project's subdirectories
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	// Sort tool counts descending
	counts := make([]ToolCount, 0, len(toolFreq))
	for name, count := range toolFreq {
		counts = append(counts, ToolCount{Name: name, Count: count})
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].Count > counts[j].Count
	})

	return &ScanResult{
		Projects:   projects,
		ToolCounts: counts,
	}, nil
}

func readScripts(dir string) ([]Script, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var scripts []Script

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name())

		commands, err := extractCommands(path)
		if err != nil {
			continue
		}

		scripts = append(scripts, Script{
			Name:     entry.Name(),
			Path:     path,
			Commands: commands,
		})
	}

	return scripts, nil
}

func extractCommands(path string) ([]string, error) {
	// Only parse shell scripts (.sh, .bash) and extensionless files
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" && ext != ".sh" && ext != ".bash" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = f.Close() }()

	var commands []string

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		// Skip empty lines, comments, shebangs
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// Skip shell control flow and syntax fragments
		if isShellSyntax(line) {
			continue
		}

		// Skip lines that look like code rather than commands
		if isCodeFragment(line) {
			continue
		}

		// Extract the base command from each statement in a pipeline or chain
		statements := splitStatements(line)
		for _, stmt := range statements {
			tool := extractTool(stmt)
			if tool != "" {
				commands = append(commands, tool)
			}
		}
	}

	return commands, sc.Err()
}

// isShellSyntax returns true for shell control flow keywords.
func isShellSyntax(line string) bool {
	keywords := []string{
		"if ", "then", "else", "elif ", "fi", "for ", "do", "done",
		"while ", "until ", "case ", "esac", "select ", "function ",
		"}", "{", "[[", "]]", "((", "))",
	}
	for _, kw := range keywords {
		if line == kw || strings.HasPrefix(line, kw) {
			return true
		}
	}

	return false
}

// isCodeFragment returns true for lines that are clearly code, not shell commands.
func isCodeFragment(line string) bool {
	// Heredoc markers
	if line == "EOF" || line == "EOF'" || line == "'EOF'" {
		return true
	}

	// Lines starting with non-command characters
	if len(line) > 0 {
		first := line[0]
		if first == '-' || first == ')' || first == ']' || first == '"' ||
			first == '\'' || first == '(' || first == '{' || first == '}' ||
			first == '>' || first == '<' || first == '&' || first == '|' {
			return true
		}
	}

	// Go/Python/JS code patterns
	codePatterns := []string{
		"func ", "func(", "type ", "struct ", "interface ", "import ",
		"package ", "var ", "const ", "return ", "defer ", "range ",
		"def ", "class ", "from ", "import(", "require(",
		"fmt.", "log.", "os.", "err.", "ctx.",
	}
	for _, p := range codePatterns {
		if strings.HasPrefix(line, p) {
			return true
		}
	}

	return false
}

// splitStatements splits a line by pipes and logical operators (&&, ||, ;).
func splitStatements(line string) []string {
	// Simple split — handles most common cases
	var parts []string

	for part := range strings.SplitSeq(line, "&&") {
		for p := range strings.SplitSeq(part, "||") {
			for s := range strings.SplitSeq(p, ";") {
				for piped := range strings.SplitSeq(s, "|") {
					trimmed := strings.TrimSpace(piped)
					if trimmed != "" {
						parts = append(parts, trimmed)
					}
				}
			}
		}
	}

	return parts
}

// extractTool gets the base tool name from a command statement.
func extractTool(stmt string) string {
	// Strip leading variable assignments (VAR=val cmd ...)
	for strings.Contains(stmt, "=") {
		parts := strings.SplitN(stmt, " ", 2)
		if len(parts) < 2 {
			return ""
		}

		if strings.Contains(parts[0], "=") && !strings.HasPrefix(parts[0], "-") {
			stmt = strings.TrimSpace(parts[1])
			continue
		}

		break
	}

	// Handle cd, export, set, etc.
	fields := strings.Fields(stmt)
	if len(fields) == 0 {
		return ""
	}

	cmd := fields[0]

	// For "omni <subcmd>", track as "omni <subcmd>"
	if cmd == "omni" && len(fields) > 1 {
		return "omni " + fields[1]
	}

	// For "go <subcmd>", "git <subcmd>", "gh <subcmd>", "docker <subcmd>", etc.
	multiWordTools := []string{"go", "git", "gh", "docker", "kubectl", "task", "terraform", "tf", "npm", "cargo", "pip"}
	for _, t := range multiWordTools {
		if cmd == t && len(fields) > 1 && !strings.HasPrefix(fields[1], "-") {
			return cmd + " " + fields[1]
		}
	}

	// Skip builtins that aren't interesting
	skipList := map[string]bool{
		"cd": true, "export": true, "set": true, "echo": true,
		"printf": true, "read": true, "shift": true, "exit": true,
		"return": true, "source": true, ".": true, "local": true,
		"declare": true, "unset": true, "eval": true, "exec": true,
		"true": true, "false": true, "test": true, "[": true,
		"sleep": true, "wait": true, "trap": true, "break": true,
		"continue": true, "readonly": true, "typeset": true,
	}
	if skipList[cmd] {
		return ""
	}

	// Validate: command must start with a letter and contain only valid chars
	if !isValidCommand(cmd) {
		return ""
	}

	return cmd
}

// isValidCommand checks if the string looks like a real command name.
func isValidCommand(cmd string) bool {
	if len(cmd) == 0 {
		return false
	}

	// Must start with a lowercase letter (reject capitalized words like Add, FAIL, Replace)
	if cmd[0] < 'a' || cmd[0] > 'z' {
		return false
	}

	// Reject file paths (contains / or ends with common extensions)
	if strings.Contains(cmd, "/") {
		return false
	}

	for _, ext := range []string{".go", ".md", ".yml", ".yaml", ".json", ".toml", ".txt", ".sh", ".js", ".ts", ".py", ".mod", ".sum", ".proto", ".html", ".css", ".svg"} {
		if strings.HasSuffix(cmd, ext) {
			return false
		}
	}

	// Must contain only alphanumeric, dash, underscore, dot, space
	for _, c := range cmd {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') &&
			c != '-' && c != '_' && c != '.' && c != ' ' {
			return false
		}
	}

	// Reject common non-command tokens that slip through
	noise := map[string]bool{
		"do": true, "with": true, "and": true, "the": true, "not": true,
		"via": true, "got": true, "err": true, "let": true, "that": true,
		"to": true, "total": true, "count": true, "n": true, "d": true,
		"ch": true, "tr": true, "expected": true, "error": true, "stmts": true,
		"option": true, "update": true, "start": true, "border": true, "url": true,
		"kind": true, "add": true,
	}
	if noise[cmd] {
		return false
	}

	// Reject if it looks like a CSS class (contains px-, bg-, text-, etc.)
	if strings.Contains(cmd, "px-") || strings.Contains(cmd, "bg-") {
		return false
	}

	return true
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
