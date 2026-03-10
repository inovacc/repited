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

// ScanOptions configures the directory scan.
type ScanOptions struct {
	MaxDepth int
	Exclude  []string // directory names to skip (e.g., "node_modules", "vendor", ".cache")
}

// DefaultExcludes are directory names skipped during every scan.
var DefaultExcludes = []string{
	"node_modules", "vendor", ".cache", "__pycache__", ".venv",
	"target", "build", "dist", ".gradle", ".maven",
}

// Scan walks rootDir looking for directories containing both .git and .scripts.
func Scan(rootDir string, opts ScanOptions) (*ScanResult, error) {
	var projects []Project

	toolFreq := make(map[string]int)

	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolving root dir: %w", err)
	}

	// Build the exclude set from defaults + user-supplied patterns.
	excludeSet := make(map[string]struct{}, len(DefaultExcludes)+len(opts.Exclude))
	for _, e := range DefaultExcludes {
		excludeSet[e] = struct{}{}
	}

	for _, e := range opts.Exclude {
		excludeSet[e] = struct{}{}
	}

	rootDepth := strings.Count(filepath.ToSlash(rootDir), "/")

	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries
		}

		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		// Skip excluded directories (but never skip the root itself).
		if path != rootDir {
			if _, ok := excludeSet[name]; ok {
				return filepath.SkipDir
			}
		}

		// Skip hidden dirs (except the ones we care about)
		if strings.HasPrefix(name, ".") && name != ".scripts" && name != ".git" {
			return filepath.SkipDir
		}

		// Enforce max depth
		if opts.MaxDepth > 0 {
			currentDepth := strings.Count(filepath.ToSlash(path), "/") - rootDepth
			if currentDepth > opts.MaxDepth {
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
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case "", ".sh", ".bash":
		return extractShellCommands(path)
	case ".ps1":
		return extractPowerShellCommands(path)
	case ".py":
		return extractPythonCommands(path)
	default:
		return nil, nil
	}
}

func extractShellCommands(path string) ([]string, error) {
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

// extractPowerShellCommands parses a .ps1 file and extracts tool commands.
func extractPowerShellCommands(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := preprocessPowerShell(string(data))

	var commands []string

	for _, line := range lines {
		statements := splitStatements(line)
		for _, stmt := range statements {
			tool := extractPowerShellTool(stmt)
			if tool != "" {
				commands = append(commands, tool)
			}
		}
	}

	return commands, nil
}

// preprocessPowerShell handles PowerShell-specific syntax: removes comments
// (single-line # and multi-line <# ... #>), joins backtick-continued lines,
// and returns cleaned lines ready for command extraction.
func preprocessPowerShell(content string) []string {
	// Remove multi-line block comments <# ... #>
	for {
		start := strings.Index(content, "<#")
		if start == -1 {
			break
		}

		end := strings.Index(content[start+2:], "#>")
		if end == -1 {
			// Unterminated block comment — remove the rest
			content = content[:start]

			break
		}

		content = content[:start] + content[start+2+end+2:]
	}

	rawLines := strings.Split(content, "\n")

	var lines []string

	var continued string

	for _, raw := range rawLines {
		line := strings.TrimSpace(raw)

		// Skip empty lines and single-line comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle backtick line continuation
		if trimmed, found := strings.CutSuffix(line, "`"); found {
			continued += trimmed + " "

			continue
		}

		if continued != "" {
			line = continued + line
			continued = ""
		}

		// Handle invocation operator before code fragment check,
		// since & at the start would be flagged by isCodeFragment.
		line = stripInvocationOperator(line)
		if line == "" {
			continue
		}

		// Skip lines that look like code rather than commands
		if isCodeFragment(line) {
			continue
		}

		// Skip PowerShell control flow
		if isPowerShellSyntax(line) {
			continue
		}

		lines = append(lines, line)
	}

	return lines
}

// isPowerShellSyntax returns true for PowerShell control flow keywords.
func isPowerShellSyntax(line string) bool {
	keywords := []string{
		"if ", "if(", "else", "elseif ", "elseif(",
		"foreach ", "foreach(", "for ", "for(",
		"while ", "while(", "do ", "do{",
		"switch ", "switch(",
		"try", "catch", "finally",
		"param(", "param (",
		"begin", "process", "end",
		"}", "{",
	}

	lower := strings.ToLower(line)
	for _, kw := range keywords {
		if lower == kw || strings.HasPrefix(lower, kw) {
			return true
		}
	}

	return false
}

// PowerShell cmdlet builtins to skip (common Verb-Noun patterns).
var psCmdletPrefixes = []string{
	"write-", "get-", "set-", "new-", "remove-", "add-", "clear-",
	"invoke-", "start-", "stop-", "test-", "out-", "select-",
	"where-", "sort-", "group-", "measure-", "format-", "export-",
	"import-", "convertto-", "convertfrom-", "update-", "enable-",
	"disable-", "register-", "unregister-", "copy-", "move-",
	"rename-", "read-", "send-", "receive-", "wait-",
}

// stripInvocationOperator removes the PowerShell & invocation operator prefix
// and unquotes the command name if quoted (e.g., & "docker" ps -> docker ps).
func stripInvocationOperator(line string) string {
	if !strings.HasPrefix(line, "&") {
		return line
	}

	line = strings.TrimSpace(line[1:])

	// If the first token is quoted, unquote just that token
	if len(line) > 0 && (line[0] == '"' || line[0] == '\'') {
		quote := line[0]
		end := strings.IndexByte(line[1:], quote)

		if end != -1 {
			unquoted := line[1 : end+1]
			rest := line[end+2:]
			line = unquoted + rest
		}
	}

	return strings.TrimSpace(line)
}

// extractPowerShellTool extracts a tool name from a PowerShell statement.
func extractPowerShellTool(stmt string) string {
	stmt = strings.TrimSpace(stmt)
	if stmt == "" {
		return ""
	}

	// Handle invocation operator (may still appear inside split statements)
	stmt = stripInvocationOperator(stmt)

	// Strip leading $variable assignments like $var = command
	if strings.HasPrefix(stmt, "$") {
		eqIdx := strings.Index(stmt, "=")
		if eqIdx != -1 {
			stmt = strings.TrimSpace(stmt[eqIdx+1:])
		} else {
			// Bare $variable reference — skip
			return ""
		}
	}

	// Skip lines that start with $ (variable references remaining after strip)
	if strings.HasPrefix(stmt, "$") {
		return ""
	}

	fields := strings.Fields(stmt)
	if len(fields) == 0 {
		return ""
	}

	cmd := fields[0]

	// Skip PowerShell cmdlets (Verb-Noun pattern)
	cmdLower := strings.ToLower(cmd)
	for _, prefix := range psCmdletPrefixes {
		if strings.HasPrefix(cmdLower, prefix) {
			return ""
		}
	}

	// Skip PowerShell builtins/aliases that don't map to real tools
	psSkip := map[string]bool{
		"cd": true, "echo": true, "exit": true, "return": true,
		"throw": true, "cls": true, "pushd": true, "popd": true,
		"mkdir": true, "rmdir": true, "del": true, "copy": true,
		"move": true, "type": true, "set-location": true,
	}

	if psSkip[cmdLower] {
		return ""
	}

	// Delegate to the shared tool extraction logic
	return extractTool(stmt)
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

// extractPythonCommands parses a .py file and extracts tool commands from
// subprocess calls, os.system/os.popen calls, and Jupyter-style !commands.
func extractPythonCommands(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := preprocessPython(string(data))

	var commands []string

	for _, line := range lines {
		cmds := extractPythonLine(line)
		commands = append(commands, cmds...)
	}

	return commands, nil
}

// preprocessPython removes comments, docstrings, and Python syntax lines,
// returning only lines that may contain shell command invocations.
func preprocessPython(content string) []string {
	rawLines := strings.Split(content, "\n")

	var lines []string

	inDocstring := false
	docstringDelim := ""

	for _, raw := range rawLines {
		line := strings.TrimSpace(raw)

		// Handle docstring blocks (""" or ''')
		if inDocstring {
			if strings.Contains(line, docstringDelim) {
				inDocstring = false
			}

			continue
		}

		// Check for docstring start
		for _, delim := range []string{`"""`, `'''`} {
			if strings.Contains(line, delim) {
				// Count occurrences — if odd, we're entering a docstring
				count := strings.Count(line, delim)
				if count == 1 {
					inDocstring = true
					docstringDelim = delim

					// If this line also has content before the docstring, skip the whole line
					break
				}
				// count >= 2 means open+close on same line — not a block docstring
			}
		}

		if inDocstring {
			continue
		}

		// Skip empty lines and single-line comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip Python keywords/syntax lines
		if isPythonSyntax(line) {
			continue
		}

		lines = append(lines, line)
	}

	return lines
}

// isPythonSyntax returns true for lines that are Python control flow or declarations.
func isPythonSyntax(line string) bool {
	keywords := []string{
		"import ", "from ", "def ", "class ", "return ", "return\t",
		"if ", "if(", "elif ", "elif(", "else:", "else :",
		"for ", "for(", "while ", "while(",
		"try:", "try :", "except ", "except:", "finally:", "finally :",
		"with ", "raise ", "yield ", "yield(",
		"pass", "break", "continue",
		"assert ", "assert(",
		"print(", "print ",
		"@",
	}

	for _, kw := range keywords {
		if line == kw || strings.HasPrefix(line, kw) {
			return true
		}
	}

	// Bare "return" on its own
	if line == "return" || line == "pass" || line == "break" || line == "continue" {
		return true
	}

	return false
}

// extractPythonLine extracts tool commands from a single preprocessed Python line.
func extractPythonLine(line string) []string {
	var commands []string

	// Handle Jupyter-style !command
	if strings.HasPrefix(line, "!") {
		cmd := strings.TrimSpace(line[1:])
		if cmd != "" {
			tool := extractTool(cmd)
			if tool != "" {
				commands = append(commands, tool)
			}
		}

		return commands
	}

	// Handle subprocess.run/call/check_call/check_output/Popen with list args
	// e.g., subprocess.run(["git", "push"]) or subprocess.call(["kubectl", "apply", "-f"])
	subprocessPrefixes := []string{
		"subprocess.run(", "subprocess.call(", "subprocess.check_call(",
		"subprocess.check_output(", "subprocess.Popen(",
	}

	for _, prefix := range subprocessPrefixes {
		_, after, ok := strings.Cut(line, prefix)
		if !ok {
			continue
		}

		cmd := extractCommandFromSubprocess(after)

		if cmd != "" {
			tool := extractTool(cmd)
			if tool != "" {
				commands = append(commands, tool)
			}
		}
	}

	if len(commands) > 0 {
		return commands
	}

	// Handle os.system("command") and os.popen("command")
	osPrefixes := []string{"os.system(", "os.popen("}

	for _, prefix := range osPrefixes {
		_, after, ok := strings.Cut(line, prefix)
		if !ok {
			continue
		}

		cmd := extractQuotedString(after)

		if cmd != "" {
			// The quoted string is a shell command — split and extract
			stmts := splitStatements(cmd)
			for _, stmt := range stmts {
				tool := extractTool(stmt)
				if tool != "" {
					commands = append(commands, tool)
				}
			}
		}
	}

	return commands
}

// extractCommandFromSubprocess extracts a command string from subprocess-style
// list arguments like ["git", "push", "--force"] or a string argument like "git push".
func extractCommandFromSubprocess(after string) string {
	after = strings.TrimSpace(after)

	// List form: ["git", "push", "--force"]
	if strings.HasPrefix(after, "[") {
		end := strings.Index(after, "]")
		if end == -1 {
			return ""
		}

		listContent := after[1:end]

		return parseStringList(listContent)
	}

	// String form: "git push --force" or 'git push'
	return extractQuotedString(after)
}

// parseStringList parses a Python list of string literals like "git", "push", "--force"
// and returns them joined as a command string.
func parseStringList(content string) string {
	var parts []string

	for part := range strings.SplitSeq(content, ",") {
		part = strings.TrimSpace(part)
		// Strip quotes
		if len(part) >= 2 {
			if (part[0] == '"' && part[len(part)-1] == '"') ||
				(part[0] == '\'' && part[len(part)-1] == '\'') {
				part = part[1 : len(part)-1]
			}
		}

		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, " ")
}

// extractQuotedString extracts the content of the first quoted string (single or double)
// from the given text.
func extractQuotedString(text string) string {
	text = strings.TrimSpace(text)

	if len(text) == 0 {
		return ""
	}

	// Skip f-strings — they contain interpolation and are unreliable
	if strings.HasPrefix(text, "f\"") || strings.HasPrefix(text, "f'") {
		return ""
	}

	var quote byte

	switch text[0] {
	case '"':
		quote = '"'
	case '\'':
		quote = '\''
	}

	if quote == 0 {
		return ""
	}

	end := strings.IndexByte(text[1:], quote)
	if end == -1 {
		return ""
	}

	return text[1 : end+1]
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
