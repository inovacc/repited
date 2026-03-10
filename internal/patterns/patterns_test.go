package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/repited/internal/store"
)

func testPatternStore(t *testing.T) *PatternStore {
	t.Helper()
	dir := t.TempDir()

	return NewPatternStore(dir)
}

// ── NewPatternStore ──

func TestNewPatternStore(t *testing.T) {
	ps := NewPatternStore("/tmp/patterns")
	if ps == nil {
		t.Fatal("expected non-nil PatternStore")
	}
}

// ── Init ──

func TestInit(t *testing.T) {
	ps := testPatternStore(t)
	if err := ps.Init(); err != nil {
		t.Fatal(err)
	}

	// Verify files created
	patternsFile := filepath.Join(ps.dir, "builtin-patterns.json")
	rulesFile := filepath.Join(ps.dir, "builtin-rules.json")

	if _, err := os.Stat(patternsFile); err != nil {
		t.Errorf("builtin-patterns.json should exist: %v", err)
	}

	if _, err := os.Stat(rulesFile); err != nil {
		t.Errorf("builtin-rules.json should exist: %v", err)
	}

	// Verify JSON is valid
	data, err := os.ReadFile(patternsFile)
	if err != nil {
		t.Fatal(err)
	}

	var patterns []Pattern
	if err := json.Unmarshal(data, &patterns); err != nil {
		t.Fatalf("invalid patterns JSON: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("expected builtin patterns")
	}
}

// ── BuiltinPatterns ──

func TestBuiltinPatterns(t *testing.T) {
	patterns := BuiltinPatterns()
	if len(patterns) != 10 {
		t.Errorf("expected 10 builtin patterns, got %d", len(patterns))
	}

	for _, p := range patterns {
		if p.ID == "" {
			t.Error("pattern has empty ID")
		}

		if p.Name == "" {
			t.Error("pattern has empty Name")
		}

		if p.Source != "builtin" {
			t.Errorf("pattern %q source = %q, want builtin", p.ID, p.Source)
		}

		if p.Confidence < 0 || p.Confidence > 1.0 {
			t.Errorf("pattern %q confidence = %f, want [0,1]", p.ID, p.Confidence)
		}

		if len(p.Steps) == 0 {
			t.Errorf("pattern %q has no steps", p.ID)
		}

		for _, s := range p.Steps {
			if s.Tool == "" {
				t.Errorf("pattern %q has step with empty tool", p.ID)
			}

			if s.Order <= 0 {
				t.Errorf("pattern %q step %q has order %d", p.ID, s.Tool, s.Order)
			}
		}
	}
}

// ── BuiltinRules ──

func TestBuiltinRules(t *testing.T) {
	rules := BuiltinRules()
	if len(rules) < 10 {
		t.Errorf("expected >= 10 builtin rules, got %d", len(rules))
	}

	validCategories := map[string]bool{
		"pre-commit": true, "quality": true, "security": true, "convention": true,
	}
	validSeverities := map[string]bool{
		"error": true, "warning": true, "info": true,
	}

	for _, r := range rules {
		if r.ID == "" {
			t.Error("rule has empty ID")
		}

		if !validCategories[r.Category] {
			t.Errorf("rule %q has invalid category %q", r.ID, r.Category)
		}

		if !validSeverities[r.Severity] {
			t.Errorf("rule %q has invalid severity %q", r.ID, r.Severity)
		}

		if r.Check == "" {
			t.Errorf("rule %q has empty check", r.ID)
		}

		if r.Fix == "" {
			t.Errorf("rule %q has empty fix", r.ID)
		}
	}
}

// ── LoadPatterns / LoadRules ──

func TestLoadPatternsAfterInit(t *testing.T) {
	ps := testPatternStore(t)
	if err := ps.Init(); err != nil {
		t.Fatal(err)
	}

	patterns, err := ps.LoadPatterns()
	if err != nil {
		t.Fatal(err)
	}

	if len(patterns) == 0 {
		t.Error("expected patterns after Init()")
	}

	// Should not include rules
	for _, p := range patterns {
		if p.Category == "pre-commit" && p.ID == "" {
			t.Error("LoadPatterns should not return rules")
		}
	}
}

func TestLoadRulesAfterInit(t *testing.T) {
	ps := testPatternStore(t)
	if err := ps.Init(); err != nil {
		t.Fatal(err)
	}

	rules, err := ps.LoadRules()
	if err != nil {
		t.Fatal(err)
	}

	if len(rules) == 0 {
		t.Error("expected rules after Init()")
	}
}

func TestLoadPatternsEmptyDir(t *testing.T) {
	ps := testPatternStore(t)
	// Don't init — dir exists but no files
	patterns, err := ps.LoadPatterns()
	if err != nil {
		t.Fatal(err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns from empty dir, got %d", len(patterns))
	}
}

func TestLoadPatternsNonexistentDir(t *testing.T) {
	ps := NewPatternStore("/nonexistent/path/patterns")

	patterns, err := ps.LoadPatterns()
	if err != nil {
		t.Fatal(err)
	}

	if patterns != nil {
		t.Errorf("expected nil for nonexistent dir, got %v", patterns)
	}
}

func TestLoadPatternsSingleItem(t *testing.T) {
	ps := testPatternStore(t)
	// Write a single pattern (not array)
	p := Pattern{ID: "single-test", Name: "single", Source: "test", Confidence: 0.5}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(ps.dir, "single-pattern.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	patterns, err := ps.LoadPatterns()
	if err != nil {
		t.Fatal(err)
	}

	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}

	if patterns[0].ID != "single-test" {
		t.Errorf("pattern ID = %q, want %q", patterns[0].ID, "single-test")
	}
}

func TestLoadPatternsSkipsInvalidJSON(t *testing.T) {
	ps := testPatternStore(t)
	// Write invalid JSON
	if err := os.WriteFile(filepath.Join(ps.dir, "bad-patterns.json"), []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write valid pattern
	data, err := json.Marshal([]Pattern{{ID: "good", Name: "good"}})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(ps.dir, "good-patterns.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	patterns, err := ps.LoadPatterns()
	if err != nil {
		t.Fatal(err)
	}

	if len(patterns) != 1 {
		t.Errorf("expected 1 pattern (skipping invalid), got %d", len(patterns))
	}
}

func TestLoadPatternsFiltersRules(t *testing.T) {
	ps := testPatternStore(t)
	if err := ps.Init(); err != nil {
		t.Fatal(err)
	}

	patterns, err := ps.LoadPatterns()
	if err != nil {
		t.Fatal(err)
	}

	rules, err := ps.LoadRules()
	if err != nil {
		t.Fatal(err)
	}

	// Patterns and rules should not overlap in source files
	if len(patterns) == 0 {
		t.Error("expected patterns")
	}

	if len(rules) == 0 {
		t.Error("expected rules")
	}
}

// ── SuggestFlows ──

func TestSuggestFlowsGoProject(t *testing.T) {
	ps := testPatternStore(t)
	if err := ps.Init(); err != nil {
		t.Fatal(err)
	}

	// Create a fake Go project
	projDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projDir, "go.mod"), []byte("module test"), 0o644); err != nil {
		t.Fatal(err)
	}

	suggestions, err := ps.SuggestFlows(projDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(suggestions) == 0 {
		t.Error("expected suggestions for Go project")
	}

	// Should be sorted by confidence desc
	for i := 1; i < len(suggestions); i++ {
		if suggestions[i].Confidence > suggestions[i-1].Confidence {
			t.Error("suggestions should be sorted by confidence descending")
			break
		}
	}
}

func TestSuggestFlowsEmptyProject(t *testing.T) {
	ps := testPatternStore(t)
	if err := ps.Init(); err != nil {
		t.Fatal(err)
	}

	projDir := t.TempDir()

	suggestions, err := ps.SuggestFlows(projDir)
	if err != nil {
		t.Fatal(err)
	}
	// Some patterns have no strict requirements (kubernetes), so we may still get some
	t.Logf("got %d suggestions for empty project", len(suggestions))
}

// ── matchesProject ──

func TestMatchesProject(t *testing.T) {
	goDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module test"), 0o644); err != nil {
		t.Fatal(err)
	}

	nodeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(nodeDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	emptyDir := t.TempDir()

	tests := []struct {
		name string
		pat  Pattern
		dir  string
		want bool
	}{
		{
			name: "go pattern matches go project",
			pat:  Pattern{Tags: []string{"go"}},
			dir:  goDir,
			want: true,
		},
		{
			name: "go pattern rejects empty project",
			pat:  Pattern{Tags: []string{"go"}},
			dir:  emptyDir,
			want: false,
		},
		{
			name: "node pattern matches node project",
			pat:  Pattern{Tags: []string{"node"}},
			dir:  nodeDir,
			want: true,
		},
		{
			name: "node pattern rejects go project",
			pat:  Pattern{Tags: []string{"node"}},
			dir:  goDir,
			want: false,
		},
		{
			name: "no tags matches any project",
			pat:  Pattern{Tags: nil},
			dir:  emptyDir,
			want: true,
		},
		{
			name: "kubernetes tag matches any (no filter)",
			pat:  Pattern{Tags: []string{"kubernetes"}},
			dir:  emptyDir,
			want: true,
		},
		{
			name: "require missing file",
			pat:  Pattern{Steps: []Step{{Require: "missing.txt"}}},
			dir:  emptyDir,
			want: false,
		},
		{
			name: "require existing file",
			pat:  Pattern{Steps: []Step{{Require: "go.mod"}}},
			dir:  goDir,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesProject(tt.pat, tt.dir); got != tt.want {
				t.Errorf("matchesProject() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ── isGuardPair ──

func TestIsGuardPair(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{"go mod tidy", "go build", true},
		{"go mod tidy", "go build ./...", true},
		{"go mod tidy", "go test", true},
		{"go build", "go test", true},
		{"go build", "go vet", true},
		{"git add", "git commit", true},
		{"git add .", "git commit", true},
		{"git commit", "git push", true},
		{"npm install", "npm test", true},
		{"cargo build", "cargo test", true},
		{"docker build", "docker push", true},
		{"terraform init", "terraform plan", true},
		{"terraform plan", "terraform apply", true},

		// Invalid pairs
		{"go test", "go mod tidy", false},
		{"git push", "git commit", false},
		{"unknown", "go build", false},
		{"", "go build", false},
	}
	for _, tt := range tests {
		t.Run(tt.from+" → "+tt.to, func(t *testing.T) {
			if got := isGuardPair(tt.from, tt.to); got != tt.want {
				t.Errorf("isGuardPair(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// ── categorizeChain ──

func TestCategorizeChain(t *testing.T) {
	tests := []struct {
		name  string
		chain []string
		want  string
	}{
		{"deploy", []string{"docker build", "docker push"}, "deploy"},
		{"flow", []string{"go build", "go test", "git commit"}, "flow"},
		{"test", []string{"go build", "go test"}, "test"},
		{"pure git", []string{"git add", "git commit", "git push"}, "flow"},
		{"empty", []string{}, "flow"},
		{"k8s deploy", []string{"kubectl apply", "kubectl rollout"}, "deploy"},
		{"terraform", []string{"terraform init", "terraform apply"}, "deploy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := categorizeChain(tt.chain); got != tt.want {
				t.Errorf("categorizeChain(%v) = %q, want %q", tt.chain, got, tt.want)
			}
		})
	}
}

// ── chainTags ──

func TestChainTags(t *testing.T) {
	tags := chainTags([]string{"go build", "go test", "git add", "git commit"})
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %v", len(tags), tags)
	}
	// Should be sorted
	if tags[0] != "git" || tags[1] != "go" {
		t.Errorf("tags = %v, want [git, go]", tags)
	}
}

func TestChainTagsDedup(t *testing.T) {
	tags := chainTags([]string{"go build", "go test", "go vet"})
	if len(tags) != 1 || tags[0] != "go" {
		t.Errorf("tags = %v, want [go]", tags)
	}
}

func TestChainTagsEmpty(t *testing.T) {
	tags := chainTags(nil)
	if len(tags) != 0 {
		t.Errorf("expected empty tags, got %v", tags)
	}
}

// ── SanitizeID ──

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"go build", "go-build"},
		{"Go Build ./...", "go-build------"},
		{"docker/push", "docker-push"},
		{"some.tool", "some-tool"},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := SanitizeID(tt.input); got != tt.want {
				t.Errorf("SanitizeID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── detectFlowPatterns ──

func TestDetectFlowPatterns(t *testing.T) {
	seqs := []store.ToolRelation{
		{From: "go mod tidy", To: "go build", Count: 10},
		{From: "go build", To: "go test", Count: 8},
		{From: "go test", To: "git add", Count: 6},
		{From: "git add", To: "git commit", Count: 5},
	}

	patterns := detectFlowPatterns(seqs)
	if len(patterns) == 0 {
		t.Fatal("expected at least one flow pattern")
	}

	// Should have a chain starting from "go mod tidy"
	found := false

	for _, p := range patterns {
		if len(p.Steps) >= 3 {
			found = true

			if p.Category == "" {
				t.Error("pattern should have a category")
			}

			if p.Confidence <= 0 {
				t.Error("pattern should have positive confidence")
			}

			if p.Source != "scan-analysis" {
				t.Errorf("source = %q, want scan-analysis", p.Source)
			}
		}
	}

	if !found {
		t.Error("expected a chain with >= 3 steps")
	}
}

func TestDetectFlowPatternsEmpty(t *testing.T) {
	patterns := detectFlowPatterns(nil)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns from empty seqs, got %d", len(patterns))
	}
}

func TestDetectFlowPatternsShortChain(t *testing.T) {
	seqs := []store.ToolRelation{
		{From: "a", To: "b", Count: 5},
	}
	patterns := detectFlowPatterns(seqs)
	// Chain of 2 is too short (need >= 3)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for short chain, got %d", len(patterns))
	}
}

// ── detectGuardPatterns ──

func TestDetectGuardPatterns(t *testing.T) {
	seqs := []store.ToolRelation{
		{From: "go mod tidy", To: "go build", Count: 15},
		{From: "go build", To: "go test", Count: 12},
		{From: "unknown", To: "go build", Count: 20}, // not a guard pair
	}

	patterns := detectGuardPatterns(seqs)
	if len(patterns) != 2 {
		t.Fatalf("expected 2 guard patterns, got %d", len(patterns))
	}

	for _, p := range patterns {
		if p.Category != "guard" {
			t.Errorf("category = %q, want guard", p.Category)
		}

		if len(p.Steps) != 2 {
			t.Errorf("guard pattern should have 2 steps, got %d", len(p.Steps))
		}
	}
}

func TestDetectGuardPatternsLowCount(t *testing.T) {
	seqs := []store.ToolRelation{
		{From: "go mod tidy", To: "go build", Count: 5}, // below 10
	}

	patterns := detectGuardPatterns(seqs)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns (count < 10), got %d", len(patterns))
	}
}

// ── detectTeardownPatterns ──

func TestDetectTeardownPatterns(t *testing.T) {
	positions := []store.WorkflowStep{
		{Tool: "git push", Position: "last", Count: 15},
		{Tool: "echo done", Position: "last", Count: 5},  // below 10
		{Tool: "go build", Position: "first", Count: 20}, // not last
	}

	patterns := detectTeardownPatterns(positions)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 teardown pattern, got %d", len(patterns))
	}

	if patterns[0].Steps[0].Tool != "git push" {
		t.Errorf("teardown tool = %q, want git push", patterns[0].Steps[0].Tool)
	}
}

// ── detectClusterPatterns ──

func TestDetectClusterPatterns(t *testing.T) {
	clusters := []store.ToolCluster{
		{
			Category: "Go",
			Tools: []store.ClusterTool{
				{Tool: "go build", Count: 15},
				{Tool: "go test", Count: 10},
			},
		},
		{
			Category: "Single",
			Tools:    []store.ClusterTool{{Tool: "lonely", Count: 5}}, // < 2 tools
		},
		{
			Category: "Low",
			Tools: []store.ClusterTool{
				{Tool: "a", Count: 5},
				{Tool: "b", Count: 5},
			}, // total < 20
		},
	}

	patterns := detectClusterPatterns(clusters)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 cluster pattern, got %d", len(patterns))
	}

	if patterns[0].Category != "setup" {
		t.Errorf("category = %q, want setup", patterns[0].Category)
	}
}

// ── max / min ──

func TestMax(t *testing.T) {
	if max(3, 5) != 5 {
		t.Error("max(3,5) should be 5")
	}

	if max(5, 3) != 5 {
		t.Error("max(5,3) should be 5")
	}

	if max(0, 0) != 0 {
		t.Error("max(0,0) should be 0")
	}
}

func TestMin(t *testing.T) {
	if min(3.0, 5.0) != 3.0 {
		t.Error("min(3,5) should be 3")
	}

	if min(5.0, 3.0) != 3.0 {
		t.Error("min(5,3) should be 3")
	}

	if min(1.0, 1.0) != 1.0 {
		t.Error("min(1,1) should be 1")
	}
}

// ── writeJSON ──

func TestWriteJSON(t *testing.T) {
	ps := testPatternStore(t)
	data := []Pattern{{ID: "test", Name: "test pattern"}}

	if err := ps.writeJSON("test.json", data); err != nil {
		t.Fatal(err)
	}

	// Verify file exists and is valid JSON
	content, err := os.ReadFile(filepath.Join(ps.dir, "test.json"))
	if err != nil {
		t.Fatal(err)
	}

	var loaded []Pattern
	if err := json.Unmarshal(content, &loaded); err != nil {
		t.Fatalf("written JSON is invalid: %v", err)
	}

	if len(loaded) != 1 || loaded[0].ID != "test" {
		t.Errorf("loaded = %v, want [{ID: test}]", loaded)
	}
}
