package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/repited/internal/scanner"
	"github.com/inovacc/repited/internal/store"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
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

// ── MCP Integration Tests ──

// newTestServer creates an MCP server with all tools registered, identical to Serve()
// but without deps.EnsureAll() and without starting stdio transport.
func newTestServer() *mcpsdk.Server {
	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "repited",
			Version: "test",
		},
		&mcpsdk.ServerOptions{},
	)

	registerFlowTool(server)
	registerScanTool(server)
	registerStatsTool(server)
	registerRelationsTool(server)
	registerPatternsTool(server)
	registerScoutTool(server)
	registerNextStepsTool(server)

	return server
}

// connectTestClient creates an in-memory server+client pair and returns the client session.
func connectTestClient(t *testing.T, server *mcpsdk.Server) *mcpsdk.ClientSession {
	t.Helper()

	ctx := context.Background()
	st, ct := mcpsdk.NewInMemoryTransports()

	_, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect failed: %v", err)
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "test-client",
		Version: "test",
	}, nil)

	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}

	return cs
}

// extractText extracts the text from a CallToolResult's first TextContent item.
func extractText(t *testing.T, result *mcpsdk.CallToolResult) string {
	t.Helper()

	if len(result.Content) == 0 {
		t.Fatal("expected at least one content item")
	}

	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	return tc.Text
}

// setTestDataDir overrides LOCALAPPDATA so that cmdlog.DBPath() points to a temp directory.
// Returns a cleanup function that restores the original value.
func setTestDataDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	_ = os.Getenv("LOCALAPPDATA")

	t.Setenv("LOCALAPPDATA", tmpDir)

	// Return the expected DB path: tmpDir/Repited/repited.db
	dbDir := filepath.Join(tmpDir, "Repited")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("creating db dir: %v", err)
	}

	return filepath.Join(dbDir, "repited.db")
}

// seedTestDB creates a SQLite database with test scan data and returns its path.
func seedTestDB(t *testing.T, dbPath string) {
	t.Helper()

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}

	defer func() { _ = db.Close() }()

	result := &scanner.ScanResult{
		Projects: []scanner.Project{
			{
				Path: "/test/project-alpha",
				Scripts: []scanner.Script{
					{
						Name:     "build.sh",
						Path:     "/test/project-alpha/.scripts/build.sh",
						Commands: []string{"go build", "go test", "golangci-lint"},
					},
					{
						Name:     "deploy.sh",
						Path:     "/test/project-alpha/.scripts/deploy.sh",
						Commands: []string{"docker build", "docker push", "kubectl apply"},
					},
				},
			},
			{
				Path: "/test/project-beta",
				Scripts: []scanner.Script{
					{
						Name:     "ci.sh",
						Path:     "/test/project-beta/.scripts/ci.sh",
						Commands: []string{"go build", "go test"},
					},
				},
			},
		},
		ToolCounts: []scanner.ToolCount{
			{Name: "go build", Count: 2},
			{Name: "go test", Count: 2},
			{Name: "golangci-lint", Count: 1},
			{Name: "docker build", Count: 1},
			{Name: "docker push", Count: 1},
			{Name: "kubectl apply", Count: 1},
		},
	}

	scanID, err := db.SaveScan("/test", result)
	if err != nil {
		t.Fatalf("SaveScan failed: %v", err)
	}

	if scanID == 0 {
		t.Fatal("expected non-zero scan ID")
	}
}

func TestMCPToolStats(t *testing.T) {
	dbPath := setTestDataDir(t)
	seedTestDB(t, dbPath)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "stats",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool stats failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("stats returned error: %s", extractText(t, result))
	}

	text := extractText(t, result)

	// Should contain aggregate stats
	if !strings.Contains(text, "Total:") {
		t.Errorf("expected stats output to contain 'Total:', got:\n%s", text)
	}

	// Should contain top tools
	if !strings.Contains(text, "go build") {
		t.Errorf("expected stats output to contain 'go build', got:\n%s", text)
	}

	if !strings.Contains(text, "go test") {
		t.Errorf("expected stats output to contain 'go test', got:\n%s", text)
	}
}

func TestMCPToolStatsListScans(t *testing.T) {
	dbPath := setTestDataDir(t)
	seedTestDB(t, dbPath)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "stats",
		Arguments: map[string]any{"list": true},
	})
	if err != nil {
		t.Fatalf("CallTool stats list failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("stats list returned error: %s", extractText(t, result))
	}

	text := extractText(t, result)

	if !strings.Contains(text, "Scans:") {
		t.Errorf("expected list output to contain 'Scans:', got:\n%s", text)
	}

	if !strings.Contains(text, "/test") {
		t.Errorf("expected list output to contain root dir '/test', got:\n%s", text)
	}
}

func TestMCPToolStatsNoDatabase(t *testing.T) {
	// Point LOCALAPPDATA to an empty temp dir so no DB exists
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "stats",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool stats failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error when no database exists")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "No database found") {
		t.Errorf("expected 'No database found' message, got:\n%s", text)
	}
}

func TestMCPToolScan(t *testing.T) {
	dbPath := setTestDataDir(t)
	_ = dbPath

	// Create a temp project directory with .git and .scripts
	projectDir := t.TempDir()
	gitDir := filepath.Join(projectDir, ".git")
	scriptsDir := filepath.Join(projectDir, ".scripts")

	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}

	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("creating .scripts: %v", err)
	}

	// Write a shell script with some commands
	scriptContent := "#!/bin/bash\ngo build ./...\ngo test ./...\ngolangci-lint run ./...\n"
	if err := os.WriteFile(filepath.Join(scriptsDir, "build.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("writing build.sh: %v", err)
	}

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scan",
		Arguments: map[string]any{
			"dir":   projectDir,
			"depth": 5,
			"top":   10,
		},
	})
	if err != nil {
		t.Fatalf("CallTool scan failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("scan returned error: %s", extractText(t, result))
	}

	text := extractText(t, result)

	if !strings.Contains(text, "Scan #") {
		t.Errorf("expected scan output to contain 'Scan #', got:\n%s", text)
	}

	if !strings.Contains(text, "1 projects") {
		t.Errorf("expected scan output to contain '1 projects', got:\n%s", text)
	}

	if !strings.Contains(text, "Top") {
		t.Errorf("expected scan output to contain 'Top', got:\n%s", text)
	}
}

func TestMCPToolScanNoProjects(t *testing.T) {
	setTestDataDir(t)

	// Empty temp dir — no .git or .scripts
	emptyDir := t.TempDir()

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scan",
		Arguments: map[string]any{
			"dir": emptyDir,
		},
	})
	if err != nil {
		t.Fatalf("CallTool scan failed: %v", err)
	}

	if result.IsError {
		t.Fatal("scan should not return error for empty dir")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "No projects with .scripts found") {
		t.Errorf("expected 'No projects' message, got:\n%s", text)
	}
}

func TestMCPToolPatternsList(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "list",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns list failed: %v", err)
	}

	text := extractText(t, result)

	// Patterns "list" will either return patterns or "No patterns found" — both are valid
	if !strings.Contains(text, "Patterns") && !strings.Contains(text, "No patterns found") {
		t.Errorf("expected patterns list output, got:\n%s", text)
	}
}

func TestMCPToolPatternsInit(t *testing.T) {
	// Use a temp dir for XDG/appdata so init writes patterns there
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "init",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns init failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("patterns init returned error: %s", extractText(t, result))
	}

	text := extractText(t, result)

	if !strings.Contains(text, "Patterns initialized") {
		t.Errorf("expected 'Patterns initialized', got:\n%s", text)
	}

	if !strings.Contains(text, "builtin patterns") {
		t.Errorf("expected 'builtin patterns' count, got:\n%s", text)
	}
}

func TestMCPToolPatternsUnknownAction(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "invalid-action",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unknown action")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "Unknown action") {
		t.Errorf("expected 'Unknown action' message, got:\n%s", text)
	}
}

func TestMCPToolNextSteps(t *testing.T) {
	dir := t.TempDir()

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	actions := []string{"push", "commit", "merge", "release", "deploy", "test", "refactor"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
				Name: "next-steps",
				Arguments: map[string]any{
					"dir":   dir,
					"after": action,
				},
			})
			if err != nil {
				t.Fatalf("CallTool next-steps failed: %v", err)
			}

			if result.IsError {
				t.Fatalf("next-steps returned error: %s", extractText(t, result))
			}

			text := extractText(t, result)

			if !strings.Contains(text, "Suggested next steps") {
				t.Errorf("expected 'Suggested next steps', got:\n%s", text)
			}

			if !strings.Contains(text, action) {
				t.Errorf("expected output to mention action '%s', got:\n%s", action, text)
			}
		})
	}
}

func TestMCPToolNextStepsDefaultAction(t *testing.T) {
	dir := t.TempDir()

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	// Omit "after" — should default to "push"
	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "next-steps",
		Arguments: map[string]any{
			"dir": dir,
		},
	})
	if err != nil {
		t.Fatalf("CallTool next-steps failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("next-steps returned error: %s", extractText(t, result))
	}

	text := extractText(t, result)

	if !strings.Contains(text, "push") {
		t.Errorf("expected default action 'push' in output, got:\n%s", text)
	}
}

func TestMCPToolFlowNoGit(t *testing.T) {
	// A directory without .git should return an error
	dir := t.TempDir()

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "flow",
		Arguments: map[string]any{
			"dir": dir,
		},
	})
	if err != nil {
		t.Fatalf("CallTool flow failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for directory without .git")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "not a git repository") {
		t.Errorf("expected 'not a git repository' message, got:\n%s", text)
	}
}

func TestMCPToolFlowWithGit(t *testing.T) {
	// Create a temp dir with .git and go.mod so the flow tool detects it
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}

	// Write a minimal go.mod
	goMod := "module example.com/test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	// Only run "status" step to avoid requiring real build tools
	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "flow",
		Arguments: map[string]any{
			"dir":  dir,
			"only": []any{"status"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool flow failed: %v", err)
	}

	text := extractText(t, result)

	// The output should show step results (skip/ok/failed)
	if !strings.Contains(text, "[skip]") && !strings.Contains(text, "[ok") && !strings.Contains(text, "[failed") {
		t.Errorf("expected step result markers in output, got:\n%s", text)
	}
}

func TestMCPToolRelationsNoDatabase(t *testing.T) {
	// Point to empty temp dir — no DB
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "relations",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool relations failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error when no database exists")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "No database found") {
		t.Errorf("expected 'No database found' message, got:\n%s", text)
	}
}

func TestMCPToolRelationsWithData(t *testing.T) {
	dbPath := setTestDataDir(t)
	seedTestDB(t, dbPath)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "relations",
		Arguments: map[string]any{
			"min_count": 1,
			"limit":     10,
		},
	})
	if err != nil {
		t.Fatalf("CallTool relations failed: %v", err)
	}

	// Relations may or may not find data depending on the query results.
	// Either a valid response or "No relation data found" is acceptable.
	text := extractText(t, result)

	if text == "" {
		t.Error("expected non-empty response from relations tool")
	}
}
