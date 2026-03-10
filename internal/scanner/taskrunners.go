package scanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// taskRunnerFileNames maps recognized task runner file names to their parser.
var taskRunnerFileNames = map[string]func(string) ([]string, error){
	"Makefile":      extractMakefileCommands,
	"makefile":      extractMakefileCommands,
	"GNUmakefile":   extractMakefileCommands,
	"justfile":      extractJustfileCommands,
	"Justfile":      extractJustfileCommands,
	".justfile":     extractJustfileCommands,
	"Taskfile.yml":  extractTaskfileCommands,
	"Taskfile.yaml": extractTaskfileCommands,
	"taskfile.yml":  extractTaskfileCommands,
}

// readTaskRunnerScripts looks for recognized task runner files in a project directory
// and returns Script entries for any found. On case-insensitive filesystems (Windows),
// deduplicates by resolved file path to avoid counting the same file multiple times.
func readTaskRunnerScripts(projectDir string) []Script {
	var scripts []Script

	seen := make(map[string]struct{})

	for name, parser := range taskRunnerFileNames {
		path := filepath.Join(projectDir, name)

		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		// Deduplicate by actual file path (handles case-insensitive FS)
		resolved, err := filepath.Abs(path)
		if err != nil {
			resolved = path
		}

		resolvedLower := strings.ToLower(resolved)
		if _, ok := seen[resolvedLower]; ok {
			continue
		}

		seen[resolvedLower] = struct{}{}

		commands, err := parser(path)
		if err != nil || len(commands) == 0 {
			continue
		}

		scripts = append(scripts, Script{
			Name:     name,
			Path:     path,
			Commands: commands,
		})
	}

	return scripts
}

// makeDirectives are Make keywords/directives that should be skipped.
var makeDirectives = map[string]bool{
	".PHONY":  true,
	"include": true,
	"define":  true,
	"endef":   true,
	"ifdef":   true,
	"ifndef":  true,
	"ifeq":    true,
	"ifneq":   true,
	"else":    true,
	"endif":   true,
	"export":  true,
	"unexport": true,
	"override": true,
	"undefine": true,
	"vpath":   true,
}

// extractMakefileCommands parses a Makefile and extracts tool commands from recipe lines.
func extractMakefileCommands(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = f.Close() }()

	var commands []string

	sc := bufio.NewScanner(f)
	inRecipe := false

	var continued string

	for sc.Scan() {
		raw := sc.Text()

		// Handle line continuation
		if trimmed, found := strings.CutSuffix(raw, "\\"); found {
			continued += trimmed + " "

			continue
		}

		if continued != "" {
			raw = continued + raw
			continued = ""
		}

		line := strings.TrimSpace(raw)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			if line == "" {
				inRecipe = false
			}

			continue
		}

		// Skip Make directives
		firstWord := strings.Fields(line)[0]
		// Remove trailing colon for directive check (e.g., ".PHONY:")
		firstWordClean := strings.TrimSuffix(firstWord, ":")

		if makeDirectives[firstWordClean] {
			continue
		}

		// Skip variable assignments (VAR = value, VAR := value, VAR ?= value, VAR += value)
		if isMakeVariableAssignment(line) {
			continue
		}

		// Detect target lines (word followed by colon, not indented)
		if !strings.HasPrefix(raw, "\t") && !strings.HasPrefix(raw, " ") && strings.Contains(line, ":") {
			// This is a target line — subsequent tab-indented lines are recipes
			inRecipe = true

			continue
		}

		// Recipe lines start with tab (or spaces in some cases, but canonical is tab)
		if inRecipe && (strings.HasPrefix(raw, "\t") || strings.HasPrefix(raw, " ")) {
			recipeLine := strings.TrimSpace(raw)

			// Strip leading @ (suppress echo) and - (ignore errors)
			recipeLine = strings.TrimLeft(recipeLine, "@-")
			recipeLine = strings.TrimSpace(recipeLine)

			if recipeLine == "" || strings.HasPrefix(recipeLine, "#") {
				continue
			}

			// Replace $(MAKE) and ${MAKE} with "make"
			recipeLine = strings.ReplaceAll(recipeLine, "$(MAKE)", "make")
			recipeLine = strings.ReplaceAll(recipeLine, "${MAKE}", "make")

			// Skip shell control flow
			if isShellSyntax(recipeLine) {
				continue
			}

			// Skip code fragments
			if isCodeFragment(recipeLine) {
				continue
			}

			statements := splitStatements(recipeLine)
			for _, stmt := range statements {
				tool := extractTool(stmt)
				if tool != "" {
					commands = append(commands, tool)
				}
			}
		}
	}

	if err = sc.Err(); err != nil {
		return nil, fmt.Errorf("reading Makefile %s: %w", path, err)
	}

	return commands, nil
}

// isMakeVariableAssignment checks if a line is a Make variable assignment.
func isMakeVariableAssignment(line string) bool {
	// Look for =, :=, ?=, += not inside a recipe
	for i, c := range line {
		switch c {
		case '=':
			return true
		case ':':
			// Check for := (simple assignment)
			if i+1 < len(line) && line[i+1] == '=' {
				return true
			}
			// Otherwise this is a target definition, not a variable
			return false
		case '?':
			if i+1 < len(line) && line[i+1] == '=' {
				return true
			}
		case '+':
			if i+1 < len(line) && line[i+1] == '=' {
				return true
			}
		case ' ', '\t':
			// Continue scanning
		default:
			// Keep scanning for operator
		}
	}

	return false
}

// justfileDirectives are justfile keywords that appear at the top level.
var justfileDirectives = map[string]bool{
	"set":     true,
	"alias":   true,
	"import":  true,
	"mod":     true,
	"export":  true,
}

// extractJustfileCommands parses a justfile and extracts tool commands from recipes.
func extractJustfileCommands(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = f.Close() }()

	var commands []string

	sc := bufio.NewScanner(f)
	inRecipe := false

	var continued string

	for sc.Scan() {
		raw := sc.Text()

		// Check if line is indented (recipe line) — must check before continuation join
		isIndented := len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t')

		// Handle line continuation with backslash
		if trimmed, found := strings.CutSuffix(strings.TrimSpace(raw), "\\"); found {
			if continued == "" && isIndented {
				// Remember that the continuation started as a recipe line
				continued = "  " // marker prefix for indentation
			}

			continued += trimmed + " "

			continue
		}

		if continued != "" {
			// Preserve indentation awareness from the first continuation line
			wasIndented := strings.HasPrefix(continued, "  ")
			full := strings.TrimSpace(continued) + " " + strings.TrimSpace(raw)
			continued = ""
			raw = full

			if wasIndented {
				isIndented = true
			}
		}

		line := strings.TrimSpace(raw)

		// Skip empty lines — they end a recipe
		if line == "" {
			inRecipe = false

			continue
		}

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		if !isIndented {
			// Top-level line — check for directives
			firstWord := strings.Fields(line)[0]

			if justfileDirectives[firstWord] {
				continue
			}

			// Check if this is a target definition (contains : not in a string)
			if strings.Contains(line, ":") {
				inRecipe = true

				continue
			}

			inRecipe = false

			continue
		}

		// Indented line — recipe line
		if !inRecipe {
			continue
		}

		recipeLine := strings.TrimSpace(raw)

		// Strip leading @ (suppress echo)
		recipeLine = strings.TrimLeft(recipeLine, "@")
		recipeLine = strings.TrimSpace(recipeLine)

		if recipeLine == "" || strings.HasPrefix(recipeLine, "#") {
			continue
		}

		// Skip shell control flow
		if isShellSyntax(recipeLine) {
			continue
		}

		// Skip code fragments
		if isCodeFragment(recipeLine) {
			continue
		}

		statements := splitStatements(recipeLine)
		for _, stmt := range statements {
			tool := extractTool(stmt)
			if tool != "" {
				commands = append(commands, tool)
			}
		}
	}

	if err = sc.Err(); err != nil {
		return nil, fmt.Errorf("reading justfile %s: %w", path, err)
	}

	return commands, nil
}

// taskfileSchema represents the structure of a Taskfile.yml.
type taskfileSchema struct {
	Tasks map[string]taskfileTask `yaml:"tasks"`
}

// taskfileTask represents a single task in Taskfile.yml.
type taskfileTask struct {
	Cmds []taskfileCmd `yaml:"cmds"`
}

// taskfileCmd represents a command in a task — can be a string or an object.
type taskfileCmd struct {
	Cmd  string `yaml:"cmd"`
	Task string `yaml:"task"`
}

// UnmarshalYAML implements custom unmarshalling for taskfileCmd to handle
// both string and object forms.
func (c *taskfileCmd) UnmarshalYAML(value *yaml.Node) error {
	// Try string form first
	if value.Kind == yaml.ScalarNode {
		c.Cmd = value.Value

		return nil
	}

	// Object form: {cmd: "...", task: "..."}
	if value.Kind == yaml.MappingNode {
		type rawCmd struct {
			Cmd  string `yaml:"cmd"`
			Task string `yaml:"task"`
		}

		var raw rawCmd

		if err := value.Decode(&raw); err != nil {
			return err
		}

		c.Cmd = raw.Cmd
		c.Task = raw.Task

		return nil
	}

	return nil
}

// extractTaskfileCommands parses a Taskfile.yml and extracts tool commands.
func extractTaskfileCommands(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf taskfileSchema

	if err = yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parsing Taskfile %s: %w", path, err)
	}

	var commands []string

	for _, task := range tf.Tasks {
		for _, cmd := range task.Cmds {
			// Handle task references — record as "task <name>"
			if cmd.Task != "" {
				tool := extractTool("task " + cmd.Task)
				if tool != "" {
					commands = append(commands, tool)
				}

				continue
			}

			if cmd.Cmd == "" {
				continue
			}

			// Parse the command string like a shell line
			extractCommandsFromShellBlock(cmd.Cmd, &commands)
		}
	}

	return commands, nil
}
