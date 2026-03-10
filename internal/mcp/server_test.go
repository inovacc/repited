package mcpserver

import (
	"context"
	"encoding/json"
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

func TestMCPToolRelationsWithExplicitScanID(t *testing.T) {
	dbPath := setTestDataDir(t)
	seedTestDB(t, dbPath)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "relations",
		Arguments: map[string]any{
			"scan_id":   1,
			"min_count": 1,
			"limit":     5,
		},
	})
	if err != nil {
		t.Fatalf("CallTool relations failed: %v", err)
	}

	text := extractText(t, result)

	if text == "" {
		t.Error("expected non-empty response")
	}
}

// ── stats with projects flag ──

func TestMCPToolStatsWithProjects(t *testing.T) {
	dbPath := setTestDataDir(t)
	seedTestDB(t, dbPath)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "stats",
		Arguments: map[string]any{
			"projects": true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool stats failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("stats returned error: %s", extractText(t, result))
	}

	text := extractText(t, result)

	if !strings.Contains(text, "Top") {
		t.Errorf("expected 'Top' in output, got:\n%s", text)
	}
}

func TestMCPToolStatsWithExplicitScanID(t *testing.T) {
	dbPath := setTestDataDir(t)
	seedTestDB(t, dbPath)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "stats",
		Arguments: map[string]any{
			"scan_id": 1,
			"top":     5,
		},
	})
	if err != nil {
		t.Fatalf("CallTool stats failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("stats returned error: %s", extractText(t, result))
	}

	text := extractText(t, result)

	if !strings.Contains(text, "Top") {
		t.Errorf("expected 'Top' in output, got:\n%s", text)
	}
}

// ── scout tool ──

func TestMCPToolScoutUnknownAction(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scout",
		Arguments: map[string]any{
			"action": "nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("CallTool scout failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unknown action")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "Unknown action") {
		t.Errorf("expected 'Unknown action', got:\n%s", text)
	}
}

func TestMCPToolScoutSearchMissingQuery(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scout",
		Arguments: map[string]any{
			"action": "search",
		},
	})
	if err != nil {
		t.Fatalf("CallTool scout failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing query")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "query") {
		t.Errorf("expected error about 'query', got:\n%s", text)
	}
}

func TestMCPToolScoutNavigateMissingURL(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scout",
		Arguments: map[string]any{
			"action": "navigate",
		},
	})
	if err != nil {
		t.Fatalf("CallTool scout failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing url")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "url") {
		t.Errorf("expected error about 'url', got:\n%s", text)
	}
}

func TestMCPToolScoutScreenshotMissingURL(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scout",
		Arguments: map[string]any{
			"action": "screenshot",
		},
	})
	if err != nil {
		t.Fatalf("CallTool scout failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing url")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "url") {
		t.Errorf("expected error about 'url', got:\n%s", text)
	}
}

func TestMCPToolScoutClickMissingTarget(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scout",
		Arguments: map[string]any{
			"action": "click",
		},
	})
	if err != nil {
		t.Fatalf("CallTool scout failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing target")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "target") {
		t.Errorf("expected error about 'target', got:\n%s", text)
	}
}

func TestMCPToolScoutMarkdownMissingURL(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scout",
		Arguments: map[string]any{
			"action": "markdown",
		},
	})
	if err != nil {
		t.Fatalf("CallTool scout failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing url")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "url") {
		t.Errorf("expected error about 'url', got:\n%s", text)
	}
}

func TestMCPToolScoutExtractMissingURL(t *testing.T) {
	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "scout",
		Arguments: map[string]any{
			"action": "extract",
		},
	})
	if err != nil {
		t.Fatalf("CallTool scout failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing url")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "url") {
		t.Errorf("expected error about 'url', got:\n%s", text)
	}
}

// ── patterns tool: detect, suggest, rules ──

func TestMCPToolPatternsDetectNoDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "detect",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns detect failed: %v", err)
	}

	if !result.IsError {
		t.Error("expected error when no database exists")
	}

	text := extractText(t, result)

	if !strings.Contains(text, "No database found") {
		t.Errorf("expected 'No database found', got:\n%s", text)
	}
}

func TestMCPToolPatternsDetectWithData(t *testing.T) {
	dbPath := setTestDataDir(t)
	seedTestDB(t, dbPath)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "detect",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns detect failed: %v", err)
	}

	// Either detects patterns or says none detected — both valid
	text := extractText(t, result)

	if !strings.Contains(text, "Detected") && !strings.Contains(text, "No new patterns detected") {
		t.Errorf("expected detection result, got:\n%s", text)
	}
}

func TestMCPToolPatternsDetectWithExplicitScanID(t *testing.T) {
	dbPath := setTestDataDir(t)
	seedTestDB(t, dbPath)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action":  "detect",
			"scan_id": 1,
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns detect failed: %v", err)
	}

	text := extractText(t, result)

	if text == "" {
		t.Error("expected non-empty response")
	}
}

func TestMCPToolPatternsSuggest(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	// Create a project dir with go.mod so patterns match
	projectDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	// Init patterns first so there are builtin patterns to suggest
	initResult, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "init",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns init failed: %v", err)
	}

	if initResult.IsError {
		t.Fatalf("patterns init returned error: %s", extractText(t, initResult))
	}

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "suggest",
			"dir":    projectDir,
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns suggest failed: %v", err)
	}

	text := extractText(t, result)

	// Either finds suggestions or says "No matching patterns"
	if !strings.Contains(text, "Suggested workflows") && !strings.Contains(text, "No matching patterns") {
		t.Errorf("expected suggest output, got:\n%s", text)
	}
}

func TestMCPToolPatternsSuggestNoPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	// Empty dir — no project markers, no patterns initialized
	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "suggest",
			"dir":    t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns suggest failed: %v", err)
	}

	text := extractText(t, result)

	if !strings.Contains(text, "No matching patterns") && !strings.Contains(text, "Suggested workflows") {
		t.Errorf("expected suggest result, got:\n%s", text)
	}
}

func TestMCPToolPatternsRules(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	// Init first to create rule files
	_, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "init",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns init failed: %v", err)
	}

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "rules",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns rules failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("patterns rules returned error: %s", extractText(t, result))
	}

	text := extractText(t, result)

	if !strings.Contains(text, "Rules (") {
		t.Errorf("expected 'Rules (' in output, got:\n%s", text)
	}

	// Should contain ON/OFF status markers
	if !strings.Contains(text, "ON ") && !strings.Contains(text, "OFF") {
		t.Errorf("expected ON/OFF status, got:\n%s", text)
	}
}

func TestMCPToolPatternsRulesNoRules(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	// Don't init — no rules exist
	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "rules",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns rules failed: %v", err)
	}

	text := extractText(t, result)

	if !strings.Contains(text, "No rules found") {
		t.Errorf("expected 'No rules found', got:\n%s", text)
	}
}

func TestMCPToolPatternsListAfterInit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	// Init to create builtin patterns
	_, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "patterns",
		Arguments: map[string]any{
			"action": "init",
		},
	})
	if err != nil {
		t.Fatalf("CallTool patterns init failed: %v", err)
	}

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

	if !strings.Contains(text, "Patterns (") {
		t.Errorf("expected 'Patterns (' in output, got:\n%s", text)
	}

	// Should list Go development flow
	if !strings.Contains(text, "Go development flow") {
		t.Errorf("expected 'Go development flow' in output, got:\n%s", text)
	}
}

// ── runCommand ──

func TestRunCommand(t *testing.T) {
	// Use a simple cross-platform command
	output, err := runCommand("go", []string{"version"}, "")
	if err != nil {
		t.Fatalf("runCommand go version failed: %v", err)
	}

	if !strings.Contains(output, "go version") {
		t.Errorf("expected 'go version' in output, got: %s", output)
	}
}

func TestRunCommandWithDir(t *testing.T) {
	dir := t.TempDir()

	output, err := runCommand("go", []string{"version"}, dir)
	if err != nil {
		t.Fatalf("runCommand with dir failed: %v", err)
	}

	if !strings.Contains(output, "go version") {
		t.Errorf("expected 'go version' in output, got: %s", output)
	}
}

func TestRunCommandFailure(t *testing.T) {
	_, err := runCommand("nonexistent-binary-xyz", []string{}, "")
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

// ── install/uninstall ──

func TestInstallGlobalCreatesConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)

	err := InstallGlobal("/usr/local/bin/repited")
	if err != nil {
		t.Fatalf("InstallGlobal failed: %v", err)
	}

	configPath := filepath.Join(tmpHome, ".claude.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var config map[string]json.RawMessage

	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	raw, ok := config["mcpServers"]
	if !ok {
		t.Fatal("expected mcpServers key in config")
	}

	var servers map[string]json.RawMessage

	if err := json.Unmarshal(raw, &servers); err != nil {
		t.Fatalf("parsing mcpServers: %v", err)
	}

	if _, ok := servers["repited"]; !ok {
		t.Error("expected 'repited' entry in mcpServers")
	}
}

func TestInstallGlobalPreservesExistingConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)

	configPath := filepath.Join(tmpHome, ".claude.json")

	// Write existing config with another server
	existing := map[string]any{
		"mcpServers": map[string]any{
			"other-tool": map[string]any{
				"command": "other-tool",
				"args":    []string{"serve"},
			},
		},
	}

	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshaling existing config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("writing existing config: %v", err)
	}

	err = InstallGlobal("/usr/local/bin/repited")
	if err != nil {
		t.Fatalf("InstallGlobal failed: %v", err)
	}

	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var config map[string]json.RawMessage

	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	var servers map[string]json.RawMessage

	if err := json.Unmarshal(config["mcpServers"], &servers); err != nil {
		t.Fatalf("parsing mcpServers: %v", err)
	}

	if _, ok := servers["other-tool"]; !ok {
		t.Error("expected 'other-tool' to be preserved")
	}

	if _, ok := servers["repited"]; !ok {
		t.Error("expected 'repited' to be added")
	}
}

func TestInstallGlobalMalformedJSON(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)

	configPath := filepath.Join(tmpHome, ".claude.json")

	if err := os.WriteFile(configPath, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatalf("writing bad config: %v", err)
	}

	err := InstallGlobal("/usr/local/bin/repited")
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestInstallProject(t *testing.T) {
	projectDir := t.TempDir()

	err := InstallProject("/usr/local/bin/repited", projectDir)
	if err != nil {
		t.Fatalf("InstallProject failed: %v", err)
	}

	configPath := filepath.Join(projectDir, ".mcp.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var config map[string]json.RawMessage

	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	var servers map[string]json.RawMessage

	if err := json.Unmarshal(config["mcpServers"], &servers); err != nil {
		t.Fatalf("parsing mcpServers: %v", err)
	}

	if _, ok := servers["repited"]; !ok {
		t.Error("expected 'repited' in mcpServers")
	}
}

func TestInstallProjectPreservesExisting(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".mcp.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other"},
		},
	}

	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	err = InstallProject("/usr/local/bin/repited", projectDir)
	if err != nil {
		t.Fatalf("InstallProject failed: %v", err)
	}

	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}

	var config map[string]json.RawMessage

	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing: %v", err)
	}

	var servers map[string]json.RawMessage

	if err := json.Unmarshal(config["mcpServers"], &servers); err != nil {
		t.Fatalf("parsing mcpServers: %v", err)
	}

	if _, ok := servers["other"]; !ok {
		t.Error("expected 'other' to be preserved")
	}

	if _, ok := servers["repited"]; !ok {
		t.Error("expected 'repited' to be added")
	}
}

func TestInstallProjectMalformedJSON(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, ".mcp.json")

	if err := os.WriteFile(configPath, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	err := InstallProject("/usr/local/bin/repited", projectDir)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestUninstallGlobalNoConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)

	// No config file — should succeed silently
	err := UninstallGlobal()
	if err != nil {
		t.Fatalf("UninstallGlobal failed: %v", err)
	}
}

func TestUninstallGlobalRemovesRepited(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)

	// Install first
	err := InstallGlobal("/usr/local/bin/repited")
	if err != nil {
		t.Fatalf("InstallGlobal failed: %v", err)
	}

	// Uninstall
	err = UninstallGlobal()
	if err != nil {
		t.Fatalf("UninstallGlobal failed: %v", err)
	}

	configPath := filepath.Join(tmpHome, ".claude.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var config map[string]json.RawMessage

	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	var servers map[string]json.RawMessage

	if err := json.Unmarshal(config["mcpServers"], &servers); err != nil {
		t.Fatalf("parsing mcpServers: %v", err)
	}

	if _, ok := servers["repited"]; ok {
		t.Error("expected 'repited' to be removed")
	}
}

func TestUninstallGlobalNoMCPServers(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)

	configPath := filepath.Join(tmpHome, ".claude.json")

	// Config without mcpServers key
	if err := os.WriteFile(configPath, []byte(`{"someKey": "value"}`), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	err := UninstallGlobal()
	if err != nil {
		t.Fatalf("UninstallGlobal failed: %v", err)
	}
}

func TestUninstallGlobalMalformedJSON(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)

	configPath := filepath.Join(tmpHome, ".claude.json")

	if err := os.WriteFile(configPath, []byte("not json"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	err := UninstallGlobal()
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestUninstallGlobalMalformedMCPServers(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)

	configPath := filepath.Join(tmpHome, ".claude.json")

	if err := os.WriteFile(configPath, []byte(`{"mcpServers": "not-an-object"}`), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	err := UninstallGlobal()
	if err == nil {
		t.Error("expected error for malformed mcpServers")
	}
}

// ── suggestNextSteps with project markers ──

func TestSuggestNextStepsPushWithGoMod(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	suggestions := suggestNextSteps("commit", dir)

	hasTestSuggestion := false

	for _, s := range suggestions {
		if strings.Contains(s.detail, "nothing is broken") || strings.Contains(s.command, "go test") {
			hasTestSuggestion = true

			break
		}
	}

	if !hasTestSuggestion {
		t.Error("expected Go test suggestion when go.mod exists")
	}
}

func TestSuggestNextStepsPushWithGoreleaser(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, ".goreleaser.yaml"), []byte("builds:\n"), 0o644); err != nil {
		t.Fatalf("writing .goreleaser.yaml: %v", err)
	}

	suggestions := suggestNextSteps("push", dir)

	hasReleaseSuggestion := false

	for _, s := range suggestions {
		if strings.Contains(s.title, "release") || strings.Contains(s.detail, "release") {
			hasReleaseSuggestion = true

			break
		}
	}

	if !hasReleaseSuggestion {
		t.Error("expected release suggestion when .goreleaser.yaml exists")
	}
}

func TestSuggestNextStepsPushWithWorkflows(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("creating workflows dir: %v", err)
	}

	suggestions := suggestNextSteps("push", dir)

	hasWorkflowSuggestion := false

	for _, s := range suggestions {
		if strings.Contains(s.title, "Monitor workflow") || strings.Contains(s.command, "gh run watch") {
			hasWorkflowSuggestion = true

			break
		}
	}

	if !hasWorkflowSuggestion {
		t.Error("expected workflow monitoring suggestion when .github/workflows exists")
	}
}

func TestSuggestNextStepsReleaseWithDockerfile(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatalf("writing Dockerfile: %v", err)
	}

	suggestions := suggestNextSteps("release", dir)

	hasDockerSuggestion := false

	for _, s := range suggestions {
		if strings.Contains(s.title, "container") || strings.Contains(s.detail, "Docker") {
			hasDockerSuggestion = true

			break
		}
	}

	if !hasDockerSuggestion {
		t.Error("expected Docker suggestion when Dockerfile exists")
	}
}

func TestSuggestNextStepsMergeWithGoMod(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	suggestions := suggestNextSteps("merge", dir)

	hasTestSuggestion := false

	for _, s := range suggestions {
		if strings.Contains(s.command, "go test") {
			hasTestSuggestion = true

			break
		}
	}

	if !hasTestSuggestion {
		t.Error("expected Go test suggestion after merge when go.mod exists")
	}
}

func TestSuggestNextStepsTestWithGoMod(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	suggestions := suggestNextSteps("test", dir)

	hasBenchmark := false

	for _, s := range suggestions {
		if strings.Contains(s.command, "go test -bench") {
			hasBenchmark = true

			break
		}
	}

	if !hasBenchmark {
		t.Error("expected benchmark suggestion after test when go.mod exists")
	}
}

// ── fileExists with real file ──

func TestFileExistsWithRealFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(filePath, []byte("test"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	if !fileExists(filePath) {
		t.Error("expected fileExists to return true for existing file")
	}
}

func TestFileExistsReturnsFalseForDir(t *testing.T) {
	dir := t.TempDir()

	if fileExists(dir) {
		t.Error("expected fileExists to return false for a directory")
	}
}

// ── flow tool with different project types ──

func TestMCPToolFlowWithPackageJSON(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatalf("writing package.json: %v", err)
	}

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	// Only run "status" to avoid needing npm
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

	if !strings.Contains(text, "[skip]") && !strings.Contains(text, "[ok") {
		t.Errorf("expected step markers, got:\n%s", text)
	}
}

func TestMCPToolFlowWithCargoToml(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0o644); err != nil {
		t.Fatalf("writing Cargo.toml: %v", err)
	}

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

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

	if !strings.Contains(text, "[skip]") && !strings.Contains(text, "[ok") {
		t.Errorf("expected step markers, got:\n%s", text)
	}
}

func TestMCPToolFlowWithCustomFiles(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "flow",
		Arguments: map[string]any{
			"dir":   dir,
			"files": "main.go cmd/",
			"only":  []any{"status"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool flow failed: %v", err)
	}

	text := extractText(t, result)

	if text == "" {
		t.Error("expected non-empty response")
	}
}

func TestMCPToolFlowWithSkip(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	server := newTestServer()
	cs := connectTestClient(t, server)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "flow",
		Arguments: map[string]any{
			"dir":  dir,
			"skip": []any{"build", "test", "lint", "vet", "tidy", "stage", "commit"},
			"only": []any{"status"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool flow failed: %v", err)
	}

	text := extractText(t, result)

	if text == "" {
		t.Error("expected non-empty response")
	}
}

// ── truncate edge cases ──

func TestTruncateExactLength(t *testing.T) {
	s := "abcde"

	result := truncate(s, 5)
	if result != s {
		t.Errorf("string at exact limit should not be truncated, got: %q", result)
	}
}

func TestTruncateEmpty(t *testing.T) {
	result := truncate("", 10)
	if result != "" {
		t.Errorf("empty string should remain empty, got: %q", result)
	}
}
