package patterns

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/inovacc/repited/internal/cmdlog"
	"github.com/inovacc/repited/internal/store"
)

// PatternsDir returns the path to the patterns directory.
func PatternsDir() string {
	return filepath.Join(cmdlog.DataDir(), "patterns")
}

// Pattern represents a detected workflow pattern.
type Pattern struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"` // "flow", "guard", "setup", "deploy", "test", "refactor"
	Steps       []Step    `json:"steps"`
	Confidence  float64   `json:"confidence"` // 0.0-1.0, how often this pattern appears
	Occurrences int       `json:"occurrences"`
	Source      string    `json:"source"` // "scan-analysis", "builtin", "user-defined"
	DetectedAt  time.Time `json:"detected_at"`
	Tags        []string  `json:"tags,omitempty"`
}

// Step is a single command in a pattern.
type Step struct {
	Tool    string `json:"tool"`              // e.g. "go build", "git add"
	OnFail  string `json:"on_fail,omitempty"` // "stop", "warn", "skip"
	Order   int    `json:"order"`
	Require string `json:"require,omitempty"` // file that must exist
}

// Rule defines a constraint or best practice that repited enforces.
type Rule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"` // "pre-commit", "quality", "security", "convention"
	Severity    string   `json:"severity"` // "error", "warning", "info"
	Check       string   `json:"check"`    // what to validate
	Fix         string   `json:"fix"`      // how to fix
	Enabled     bool     `json:"enabled"`
	Tags        []string `json:"tags,omitempty"`
}

// PatternStore manages patterns and rules on disk.
type PatternStore struct {
	dir string
}

// NewPatternStore creates a store at the given directory.
func NewPatternStore(dir string) *PatternStore {
	return &PatternStore{dir: dir}
}

// Default returns a store at the default patterns directory.
func Default() *PatternStore {
	return NewPatternStore(PatternsDir())
}

// Init creates the patterns directory and writes builtin patterns/rules.
func (ps *PatternStore) Init() error {
	if err := os.MkdirAll(ps.dir, 0o755); err != nil {
		return fmt.Errorf("creating patterns dir: %w", err)
	}

	if err := ps.writeJSON("builtin-patterns.json", BuiltinPatterns()); err != nil {
		return err
	}

	if err := ps.writeJSON("builtin-rules.json", BuiltinRules()); err != nil {
		return err
	}

	return nil
}

// DetectPatterns analyzes scan data and generates new pattern files.
func (ps *PatternStore) DetectPatterns(db *store.Store, scanID int64) ([]Pattern, error) {
	var detected []Pattern

	// 1. Detect sequential flow patterns from scan data
	seqs, err := db.ToolSequences(scanID, 3, 100)
	if err != nil {
		return nil, fmt.Errorf("querying sequences: %w", err)
	}

	flows := detectFlowPatterns(seqs)
	detected = append(detected, flows...)

	// 2. Detect guard patterns (commands that always precede others)
	guards := detectGuardPatterns(seqs)
	detected = append(detected, guards...)

	// 3. Detect teardown patterns (commands that always end scripts)
	positions, err := db.ToolPositions(scanID, 100)
	if err == nil {
		teardowns := detectTeardownPatterns(positions)
		detected = append(detected, teardowns...)
	}

	// 4. Detect cluster-based patterns
	clusters, err := db.ToolClusters(scanID)
	if err == nil {
		clusterPats := detectClusterPatterns(clusters)
		detected = append(detected, clusterPats...)
	}

	// Save detected patterns
	if len(detected) > 0 {
		filename := fmt.Sprintf("detected-%s.json", time.Now().Format("20060102-150405"))
		if err := ps.writeJSON(filename, detected); err != nil {
			return detected, fmt.Errorf("saving detected patterns: %w", err)
		}
	}

	return detected, nil
}

// LoadPatterns reads all pattern files (not rule files) from the directory.
func (ps *PatternStore) LoadPatterns() ([]Pattern, error) {
	return loadFiltered[Pattern](ps.dir, "patterns", func(name string) bool {
		return !strings.Contains(name, "rules")
	})
}

// LoadRules reads all rule files from the directory.
func (ps *PatternStore) LoadRules() ([]Rule, error) {
	return loadFiltered[Rule](ps.dir, "rules", func(name string) bool {
		return strings.Contains(name, "rules")
	})
}

// SuggestFlows returns pattern suggestions for a given project directory.
func (ps *PatternStore) SuggestFlows(projectDir string) ([]Pattern, error) {
	patterns, err := ps.LoadPatterns()
	if err != nil {
		return nil, err
	}

	var suggestions []Pattern

	for _, p := range patterns {
		if matchesProject(p, projectDir) {
			suggestions = append(suggestions, p)
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Confidence > suggestions[j].Confidence
	})

	return suggestions, nil
}

// ── pattern detection logic ──

func detectFlowPatterns(seqs []store.ToolRelation) []Pattern {
	// Build adjacency: from → [(to, count)]
	type edge struct {
		to    string
		count int
	}

	adj := make(map[string][]edge)
	for _, s := range seqs {
		adj[s.From] = append(adj[s.From], edge{to: s.To, count: s.Count})
	}

	// Find the strongest chains (greedy: follow highest-count edge)
	var patterns []Pattern

	visited := make(map[string]bool)

	// Start from tools with no strong predecessor
	inDegree := make(map[string]int)
	for _, s := range seqs {
		inDegree[s.To] += s.Count
	}

	// Sort by lowest in-degree (likely start of a flow)
	type candidate struct {
		tool    string
		inDeg   int
		outEdge int
	}

	starts := make([]candidate, 0, len(adj))

	for tool, edges := range adj {
		maxOut := 0
		for _, e := range edges {
			if e.count > maxOut {
				maxOut = e.count
			}
		}

		starts = append(starts, candidate{tool: tool, inDeg: inDegree[tool], outEdge: maxOut})
	}

	sort.Slice(starts, func(i, j int) bool {
		if starts[i].inDeg != starts[j].inDeg {
			return starts[i].inDeg < starts[j].inDeg
		}

		return starts[i].outEdge > starts[j].outEdge
	})

	for _, start := range starts {
		if visited[start.tool] {
			continue
		}

		chain := []string{start.tool}
		visited[start.tool] = true
		current := start.tool
		totalCount := 0

		for {
			edges := adj[current]
			if len(edges) == 0 {
				break
			}

			sort.Slice(edges, func(i, j int) bool {
				return edges[i].count > edges[j].count
			})

			best := edges[0]
			if visited[best.to] || best.count < 3 {
				break
			}

			chain = append(chain, best.to)
			visited[best.to] = true
			totalCount += best.count
			current = best.to
		}

		if len(chain) >= 3 {
			steps := make([]Step, len(chain))
			for i, tool := range chain {
				steps[i] = Step{Tool: tool, Order: i + 1, OnFail: "stop"}
			}

			category := categorizeChain(chain)
			patterns = append(patterns, Pattern{
				ID:          fmt.Sprintf("flow-%s-%d", chain[0], len(chain)),
				Name:        fmt.Sprintf("%s → %s (%d steps)", chain[0], chain[len(chain)-1], len(chain)),
				Description: fmt.Sprintf("Detected flow: %s", strings.Join(chain, " → ")),
				Category:    category,
				Steps:       steps,
				Confidence:  float64(totalCount) / float64(len(chain)*50),
				Occurrences: totalCount / max(len(chain)-1, 1),
				Source:      "scan-analysis",
				DetectedAt:  time.Now(),
				Tags:        chainTags(chain),
			})
		}
	}

	return patterns
}

func detectGuardPatterns(seqs []store.ToolRelation) []Pattern {
	// Find "guard" patterns: A always precedes B with high confidence
	var patterns []Pattern

	for _, s := range seqs {
		if s.Count >= 10 {
			// Check if this is a natural prerequisite
			if isGuardPair(s.From, s.To) {
				patterns = append(patterns, Pattern{
					ID:          fmt.Sprintf("guard-%s-%s", sanitizeID(s.From), sanitizeID(s.To)),
					Name:        fmt.Sprintf("%s guards %s", s.From, s.To),
					Description: fmt.Sprintf("'%s' should run before '%s' (%d occurrences)", s.From, s.To, s.Count),
					Category:    "guard",
					Steps: []Step{
						{Tool: s.From, Order: 1, OnFail: "stop"},
						{Tool: s.To, Order: 2, OnFail: "stop"},
					},
					Confidence:  min(float64(s.Count)/50.0, 1.0),
					Occurrences: s.Count,
					Source:      "scan-analysis",
					DetectedAt:  time.Now(),
					Tags:        []string{"guard", "prerequisite"},
				})
			}
		}
	}

	return patterns
}

func detectTeardownPatterns(positions []store.WorkflowStep) []Pattern {
	var patterns []Pattern

	for _, ws := range positions {
		if ws.Position == "last" && ws.Count >= 10 {
			patterns = append(patterns, Pattern{
				ID:          fmt.Sprintf("teardown-%s", sanitizeID(ws.Tool)),
				Name:        fmt.Sprintf("%s (script closer)", ws.Tool),
				Description: fmt.Sprintf("'%s' commonly ends scripts (%d times)", ws.Tool, ws.Count),
				Category:    "flow",
				Steps:       []Step{{Tool: ws.Tool, Order: 1}},
				Confidence:  min(float64(ws.Count)/30.0, 1.0),
				Occurrences: ws.Count,
				Source:      "scan-analysis",
				DetectedAt:  time.Now(),
				Tags:        []string{"teardown", "closer"},
			})
		}
	}

	return patterns
}

func detectClusterPatterns(clusters []store.ToolCluster) []Pattern {
	var patterns []Pattern

	for _, cl := range clusters {
		if len(cl.Tools) < 2 {
			continue
		}

		total := 0

		steps := make([]Step, 0, len(cl.Tools))
		for i, t := range cl.Tools {
			total += t.Count
			if i < 8 { // Cap at 8 tools per cluster
				steps = append(steps, Step{Tool: t.Tool, Order: i + 1})
			}
		}

		if total >= 20 {
			patterns = append(patterns, Pattern{
				ID:          fmt.Sprintf("cluster-%s", sanitizeID(cl.Category)),
				Name:        fmt.Sprintf("%s toolkit", cl.Category),
				Description: fmt.Sprintf("Frequently co-occurring tools in %s (%d total uses)", cl.Category, total),
				Category:    "setup",
				Steps:       steps,
				Confidence:  min(float64(total)/200.0, 1.0),
				Occurrences: total,
				Source:      "scan-analysis",
				DetectedAt:  time.Now(),
				Tags:        []string{"cluster", strings.ToLower(cl.Category)},
			})
		}
	}

	return patterns
}

// ── builtin patterns and rules ──

// BuiltinPatterns returns the default set of known workflow patterns.
func BuiltinPatterns() []Pattern {
	now := time.Now()

	return []Pattern{
		{
			ID:          "builtin-go-flow",
			Name:        "Go development flow",
			Description: "Standard Go development cycle: tidy → build → vet → test → lint → stage → commit",
			Category:    "flow",
			Steps: []Step{
				{Tool: "go mod tidy", Order: 1, OnFail: "stop"},
				{Tool: "go build ./...", Order: 2, OnFail: "stop"},
				{Tool: "go vet ./...", Order: 3, OnFail: "stop"},
				{Tool: "go test ./...", Order: 4, OnFail: "warn"},
				{Tool: "golangci-lint run ./...", Order: 5, OnFail: "warn"},
				{Tool: "git add .", Order: 6, OnFail: "stop"},
				{Tool: "git status --short", Order: 7, OnFail: "skip"},
				{Tool: "git commit", Order: 8, OnFail: "stop"},
			},
			Confidence:  1.0,
			Occurrences: 0,
			Source:      "builtin",
			DetectedAt:  now,
			Tags:        []string{"go", "ci", "commit"},
		},
		{
			ID:          "builtin-node-flow",
			Name:        "Node.js development flow",
			Description: "Standard Node.js cycle: install → test → typecheck → stage → commit",
			Category:    "flow",
			Steps: []Step{
				{Tool: "npm install", Order: 1, OnFail: "stop"},
				{Tool: "npm test", Order: 2, OnFail: "warn"},
				{Tool: "npx tsc --noEmit", Order: 3, OnFail: "warn", Require: "tsconfig.json"},
				{Tool: "git add .", Order: 4, OnFail: "stop"},
				{Tool: "git commit", Order: 5, OnFail: "stop"},
			},
			Confidence: 1.0,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"node", "typescript", "ci"},
		},
		{
			ID:          "builtin-rust-flow",
			Name:        "Rust development flow",
			Description: "Standard Rust cycle: build → test → clippy → stage → commit",
			Category:    "flow",
			Steps: []Step{
				{Tool: "cargo build", Order: 1, OnFail: "stop"},
				{Tool: "cargo test", Order: 2, OnFail: "warn"},
				{Tool: "cargo clippy", Order: 3, OnFail: "warn"},
				{Tool: "git add .", Order: 4, OnFail: "stop"},
				{Tool: "git commit", Order: 5, OnFail: "stop"},
			},
			Confidence: 1.0,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"rust", "ci"},
		},
		{
			ID:          "builtin-docker-build",
			Name:        "Docker build & push",
			Description: "Build container image, tag, and push to registry",
			Category:    "deploy",
			Steps: []Step{
				{Tool: "docker build", Order: 1, OnFail: "stop", Require: "Dockerfile"},
				{Tool: "docker tag", Order: 2, OnFail: "stop"},
				{Tool: "docker push", Order: 3, OnFail: "stop"},
			},
			Confidence: 0.9,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"docker", "deploy", "container"},
		},
		{
			ID:          "builtin-pr-flow",
			Name:        "Pull request flow",
			Description: "Create branch, commit, push, create PR",
			Category:    "flow",
			Steps: []Step{
				{Tool: "git checkout", Order: 1, OnFail: "stop"},
				{Tool: "git add .", Order: 2, OnFail: "stop"},
				{Tool: "git commit", Order: 3, OnFail: "stop"},
				{Tool: "git push", Order: 4, OnFail: "stop"},
				{Tool: "gh pr create", Order: 5, OnFail: "stop"},
			},
			Confidence: 0.85,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"git", "github", "pr"},
		},
		{
			ID:          "builtin-k8s-deploy",
			Name:        "Kubernetes deploy",
			Description: "Apply manifests, wait for rollout, verify",
			Category:    "deploy",
			Steps: []Step{
				{Tool: "kubectl apply", Order: 1, OnFail: "stop"},
				{Tool: "kubectl rollout status", Order: 2, OnFail: "warn"},
				{Tool: "kubectl get pods", Order: 3, OnFail: "skip"},
			},
			Confidence: 0.8,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"kubernetes", "deploy"},
		},
		{
			ID:          "builtin-terraform-apply",
			Name:        "Terraform apply",
			Description: "Init, validate, plan, apply infrastructure changes",
			Category:    "deploy",
			Steps: []Step{
				{Tool: "terraform init", Order: 1, OnFail: "stop"},
				{Tool: "terraform validate", Order: 2, OnFail: "stop"},
				{Tool: "terraform plan", Order: 3, OnFail: "stop"},
				{Tool: "terraform apply", Order: 4, OnFail: "stop"},
			},
			Confidence: 0.85,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"terraform", "infra", "deploy"},
		},
		{
			ID:          "builtin-go-release",
			Name:        "Go release flow",
			Description: "Tag, build with goreleaser, push release",
			Category:    "deploy",
			Steps: []Step{
				{Tool: "go test ./...", Order: 1, OnFail: "stop"},
				{Tool: "git tag", Order: 2, OnFail: "stop"},
				{Tool: "git push --tags", Order: 3, OnFail: "stop"},
				{Tool: "goreleaser release", Order: 4, OnFail: "stop", Require: ".goreleaser.yaml"},
			},
			Confidence: 0.9,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"go", "release", "goreleaser"},
		},
		{
			ID:          "builtin-refactor-safe",
			Name:        "Safe refactor",
			Description: "Verify tests pass before and after refactoring",
			Category:    "refactor",
			Steps: []Step{
				{Tool: "go test ./...", Order: 1, OnFail: "stop"},
				{Tool: "go vet ./...", Order: 2, OnFail: "stop"},
				{Tool: "git stash", Order: 3, OnFail: "skip"},
			},
			Confidence: 0.7,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"refactor", "safety"},
		},
		{
			ID:          "builtin-gh-issue-flow",
			Name:        "GitHub issue workflow",
			Description: "Create branch from issue, work, PR back",
			Category:    "flow",
			Steps: []Step{
				{Tool: "gh issue view", Order: 1, OnFail: "stop"},
				{Tool: "git checkout", Order: 2, OnFail: "stop"},
				{Tool: "git add .", Order: 3, OnFail: "stop"},
				{Tool: "git commit", Order: 4, OnFail: "stop"},
				{Tool: "git push", Order: 5, OnFail: "stop"},
				{Tool: "gh pr create", Order: 6, OnFail: "stop"},
			},
			Confidence: 0.75,
			Source:     "builtin",
			DetectedAt: now,
			Tags:       []string{"github", "issue", "pr"},
		},
	}
}

// BuiltinRules returns the default set of rules.
func BuiltinRules() []Rule {
	return []Rule{
		// Pre-commit rules
		{
			ID:          "rule-lint-before-commit",
			Name:        "Lint before commit",
			Description: "Run golangci-lint --fix before every commit to auto-fix style issues",
			Category:    "pre-commit",
			Severity:    "error",
			Check:       "golangci-lint run ./...",
			Fix:         "golangci-lint run --fix ./...",
			Enabled:     true,
			Tags:        []string{"go", "lint", "quality"},
		},
		{
			ID:          "rule-vet-before-commit",
			Name:        "Vet before commit",
			Description: "Run go vet to catch suspicious constructs before committing",
			Category:    "pre-commit",
			Severity:    "error",
			Check:       "go vet ./...",
			Fix:         "Fix the issue reported by go vet",
			Enabled:     true,
			Tags:        []string{"go", "vet", "quality"},
		},
		{
			ID:          "rule-build-before-commit",
			Name:        "Build before commit",
			Description: "Ensure the project compiles before committing",
			Category:    "pre-commit",
			Severity:    "error",
			Check:       "go build ./...",
			Fix:         "Fix compilation errors",
			Enabled:     true,
			Tags:        []string{"go", "build"},
		},
		{
			ID:          "rule-test-before-push",
			Name:        "Test before push",
			Description: "Run tests before pushing to remote",
			Category:    "pre-commit",
			Severity:    "warning",
			Check:       "go test -count=1 ./...",
			Fix:         "Fix failing tests",
			Enabled:     true,
			Tags:        []string{"go", "test"},
		},
		// Quality rules
		{
			ID:          "rule-no-todo-commit",
			Name:        "No TODO in staged changes",
			Description: "Warn when committing code with TODO comments (they should become issues)",
			Category:    "quality",
			Severity:    "warning",
			Check:       "git diff --cached | grep -i 'TODO\\|FIXME\\|HACK'",
			Fix:         "Convert TODOs to GitHub issues or remove them",
			Enabled:     true,
			Tags:        []string{"quality", "hygiene"},
		},
		{
			ID:          "rule-no-debug-commit",
			Name:        "No debug statements",
			Description: "Prevent committing debug print statements",
			Category:    "quality",
			Severity:    "warning",
			Check:       "git diff --cached | grep -i 'fmt.Println\\|log.Println\\|console.log\\|print('",
			Fix:         "Remove debug print statements or convert to proper logging",
			Enabled:     false,
			Tags:        []string{"quality", "debug"},
		},
		// Security rules
		{
			ID:          "rule-no-secrets",
			Name:        "No secrets in commits",
			Description: "Prevent committing files that may contain secrets",
			Category:    "security",
			Severity:    "error",
			Check:       "git diff --cached --name-only | grep -i '.env\\|credentials\\|secret\\|token\\|password'",
			Fix:         "Remove secrets from staged files, use environment variables instead",
			Enabled:     true,
			Tags:        []string{"security", "secrets"},
		},
		{
			ID:          "rule-no-large-files",
			Name:        "No large files",
			Description: "Prevent committing files larger than 5MB",
			Category:    "quality",
			Severity:    "warning",
			Check:       "find staged files > 5MB",
			Fix:         "Use Git LFS or exclude large files",
			Enabled:     true,
			Tags:        []string{"quality", "size"},
		},
		// Convention rules
		{
			ID:          "rule-conventional-commits",
			Name:        "Conventional commit messages",
			Description: "Commit messages should follow conventional commits format (feat:, fix:, docs:, etc.)",
			Category:    "convention",
			Severity:    "info",
			Check:       "regex: ^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\\(.+\\))?!?: .+",
			Fix:         "Use format: type(scope): description",
			Enabled:     true,
			Tags:        []string{"git", "convention"},
		},
		{
			ID:          "rule-branch-naming",
			Name:        "Branch naming convention",
			Description: "Branches should follow naming convention: feature/, fix/, docs/, chore/",
			Category:    "convention",
			Severity:    "info",
			Check:       "regex: ^(main|master|develop|feature/|fix/|docs/|chore/|release/|hotfix/).+",
			Fix:         "Rename branch: git branch -m <new-name>",
			Enabled:     false,
			Tags:        []string{"git", "convention"},
		},
		// Flow optimization rules
		{
			ID:          "rule-tidy-before-build",
			Name:        "Tidy before build",
			Description: "Always run go mod tidy before go build to ensure dependencies are clean",
			Category:    "quality",
			Severity:    "info",
			Check:       "go mod tidy && git diff --exit-code go.mod go.sum",
			Fix:         "Run go mod tidy and stage the changes",
			Enabled:     true,
			Tags:        []string{"go", "dependencies"},
		},
		{
			ID:          "rule-fmt-before-lint",
			Name:        "Format before lint",
			Description: "Run gofmt/goimports before linting to avoid format-related lint failures",
			Category:    "quality",
			Severity:    "info",
			Check:       "goimports -l .",
			Fix:         "goimports -w . && go fmt ./...",
			Enabled:     true,
			Tags:        []string{"go", "format"},
		},
	}
}

// ── helpers ──

func (ps *PatternStore) writeJSON(filename string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", filename, err)
	}

	path := filepath.Join(ps.dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

func loadFiltered[T any](dir string, kind string, filter func(string) bool) ([]T, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading %s dir: %w", kind, err)
	}

	var all []T

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		if filter != nil && !filter(entry.Name()) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		// Try array first
		var items []T
		if err := json.Unmarshal(data, &items); err == nil {
			all = append(all, items...)
			continue
		}

		// Try single item
		var item T
		if err := json.Unmarshal(data, &item); err == nil {
			all = append(all, item)
		}
	}

	return all, nil
}

func matchesProject(p Pattern, dir string) bool {
	for _, step := range p.Steps {
		if step.Require != "" {
			path := filepath.Join(dir, step.Require)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return false
			}
		}
	}
	// Check if the pattern's tags match project markers
	for _, tag := range p.Tags {
		switch tag {
		case "go":
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
				return false
			}
		case "node", "typescript":
			if _, err := os.Stat(filepath.Join(dir, "package.json")); os.IsNotExist(err) {
				return false
			}
		case "rust":
			if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); os.IsNotExist(err) {
				return false
			}
		case "docker", "container":
			if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); os.IsNotExist(err) {
				return false
			}
		case "kubernetes":
			// Don't filter — k8s manifests can be anywhere
		case "terraform", "infra":
			hasMain := false

			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".tf") {
					hasMain = true
					break
				}
			}

			if !hasMain {
				return false
			}
		}
	}

	return true
}

func isGuardPair(from, to string) bool {
	guards := map[string][]string{
		"go mod tidy":    {"go build", "go build ./...", "go test", "go test ./..."},
		"go build":       {"go test", "go test ./...", "go vet", "go vet ./..."},
		"go build ./...": {"go test ./...", "go vet ./..."},
		"go vet":         {"golangci-lint"},
		"go vet ./...":   {"golangci-lint"},
		"go test":        {"git add", "git add ."},
		"go test ./...":  {"git add", "git add ."},
		"git add":        {"git commit", "git status"},
		"git add .":      {"git commit", "git status"},
		"git commit":     {"git push"},
		"npm install":    {"npm test", "npx tsc"},
		"cargo build":    {"cargo test"},
		"cargo test":     {"cargo clippy"},
		"docker build":   {"docker tag", "docker push"},
		"terraform init": {"terraform validate", "terraform plan"},
		"terraform plan": {"terraform apply"},
		"kubectl apply":  {"kubectl rollout status"},
	}

	allowed, ok := guards[from]
	if !ok {
		return false
	}

	for _, a := range allowed {
		if strings.HasPrefix(to, a) {
			return true
		}
	}

	return false
}

func categorizeChain(chain []string) string {
	hasGit := false
	hasBuild := false
	hasDeploy := false

	for _, tool := range chain {
		if strings.HasPrefix(tool, "git ") || strings.HasPrefix(tool, "gh ") {
			hasGit = true
		}

		if strings.HasPrefix(tool, "go ") || strings.HasPrefix(tool, "cargo ") || strings.HasPrefix(tool, "npm ") {
			hasBuild = true
		}

		if strings.HasPrefix(tool, "docker ") || strings.HasPrefix(tool, "kubectl ") || strings.HasPrefix(tool, "terraform ") {
			hasDeploy = true
		}
	}

	if hasDeploy {
		return "deploy"
	}

	if hasBuild && hasGit {
		return "flow"
	}

	if hasBuild {
		return "test"
	}

	return "flow"
}

func chainTags(chain []string) []string {
	tags := make(map[string]bool)

	for _, tool := range chain {
		parts := strings.SplitN(tool, " ", 2)
		switch parts[0] {
		case "go", "golangci-lint":
			tags["go"] = true
		case "git":
			tags["git"] = true
		case "gh":
			tags["github"] = true
		case "npm", "npx":
			tags["node"] = true
		case "cargo":
			tags["rust"] = true
		case "docker":
			tags["docker"] = true
		case "kubectl":
			tags["kubernetes"] = true
		case "terraform", "tf":
			tags["terraform"] = true
		}
	}

	result := make([]string, 0, len(tags))
	for t := range tags {
		result = append(result, t)
	}

	sort.Strings(result)

	return result
}

func sanitizeID(s string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", ".", "-")
	return r.Replace(strings.ToLower(s))
}

