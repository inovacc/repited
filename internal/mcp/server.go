package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/inovacc/repited/internal/cmdlog"
	"github.com/inovacc/repited/internal/deps"
	"github.com/inovacc/repited/internal/flow"
	"github.com/inovacc/repited/internal/patterns"
	"github.com/inovacc/repited/internal/scanner"
	"github.com/inovacc/repited/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Serve runs the MCP server on stdio transport.
func Serve(ctx context.Context, version string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "repited",
			Version: version,
		},
		&mcp.ServerOptions{
			Instructions: "repited reduces repeated Claude Code workflows into single tool calls. " +
				"Use 'flow' to run build→test→lint→stage→commit pipelines, " +
				"'scan' to analyze .scripts folders, 'stats' to query scan data, " +
				"'relations' to see command co-occurrence patterns, " +
				"'scout' to run browser/web automation commands, " +
				"and 'next-steps' to suggest what to do after pushing code.",
			Logger: logger,
		},
	)

	// Auto-install missing dependencies (omni, scout)
	if installed := deps.EnsureAll(); len(installed) > 0 {
		logger.Info("auto-installed dependencies", "tools", installed)
	}

	registerFlowTool(server)
	registerScanTool(server)
	registerStatsTool(server)
	registerRelationsTool(server)
	registerPatternsTool(server)
	registerScoutTool(server)
	registerNextStepsTool(server)

	return server.Run(ctx, &mcp.StdioTransport{})
}

// ── flow tool ──

type flowInput struct {
	Dir     string   `json:"dir" jsonschema:"working directory (absolute path)"`
	Message string   `json:"message,omitempty" jsonschema:"commit message (if empty, commit step is skipped)"`
	Push    bool     `json:"push,omitempty" jsonschema:"push after committing"`
	Skip    []string `json:"skip,omitempty" jsonschema:"steps to skip (e.g. lint, test)"`
	Only    []string `json:"only,omitempty" jsonschema:"only run these steps"`
	Files   string   `json:"files,omitempty" jsonschema:"files to stage (default: .)"`
}

func registerFlowTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "flow",
		Description: "Run the full development workflow: build → test → lint → stage → commit. " +
			"Auto-detects Go, Node.js, and Rust projects. Each step depends on the previous one succeeding.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input flowInput) (*mcp.CallToolResult, any, error) {
		dir := input.Dir
		if dir == "" {
			var err error

			dir, err = os.Getwd()
			if err != nil {
				return nil, nil, fmt.Errorf("getting working directory: %w", err)
			}
		}

		dir, err := filepath.Abs(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving directory: %w", err)
		}

		files := input.Files
		if files == "" {
			files = "."
		}

		skipSet := toSet(input.Skip)
		onlySet := toSet(input.Only)

		hasGoMod := fileExists(filepath.Join(dir, "go.mod"))
		hasPackageJSON := fileExists(filepath.Join(dir, "package.json"))
		hasCargoToml := fileExists(filepath.Join(dir, "Cargo.toml"))
		hasGit := dirExists(filepath.Join(dir, ".git"))

		if !hasGit {
			return textResult(fmt.Sprintf("Error: %s is not a git repository", dir), true), nil, nil
		}

		p := flow.NewPipeline(dir, false)
		p.Quiet = true

		if hasGoMod {
			p.Add(flow.Step{Name: "go mod tidy", Cmd: "go", Args: []string{"mod", "tidy"}, Skip: shouldSkip("tidy", skipSet, onlySet)})
			p.Add(flow.Step{Name: "go build ./...", Cmd: "go", Args: []string{"build", "./..."}, Skip: shouldSkip("build", skipSet, onlySet)})
			p.Add(flow.Step{Name: "go vet ./...", Cmd: "go", Args: []string{"vet", "./..."}, Skip: shouldSkip("vet", skipSet, onlySet)})
			p.Add(flow.Step{Name: "go test ./...", Cmd: "go", Args: []string{"test", "-count=1", "./..."}, Skip: shouldSkip("test", skipSet, onlySet), OnFail: "warn"})
			p.Add(flow.Step{Name: "golangci-lint", Cmd: "golangci-lint", Args: []string{"run", "./..."}, Skip: shouldSkip("lint", skipSet, onlySet), OnFail: "warn"})
		}

		if hasPackageJSON {
			p.Add(flow.Step{Name: "npm install", Cmd: "npm", Args: []string{"install"}, Skip: shouldSkip("install", skipSet, onlySet)})
			p.Add(flow.Step{Name: "npm test", Cmd: "npm", Args: []string{"test"}, Skip: shouldSkip("test", skipSet, onlySet), OnFail: "warn"})
			p.Add(flow.Step{Name: "npx tsc --noEmit", Cmd: "npx", Args: []string{"tsc", "--noEmit"}, Skip: shouldSkip("lint", skipSet, onlySet), OnFail: "warn", Require: "tsconfig.json"})
		}

		if hasCargoToml {
			p.Add(flow.Step{Name: "cargo build", Cmd: "cargo", Args: []string{"build"}, Skip: shouldSkip("build", skipSet, onlySet)})
			p.Add(flow.Step{Name: "cargo test", Cmd: "cargo", Args: []string{"test"}, Skip: shouldSkip("test", skipSet, onlySet), OnFail: "warn"})
			p.Add(flow.Step{Name: "cargo clippy", Cmd: "cargo", Args: []string{"clippy", "--", "-D", "warnings"}, Skip: shouldSkip("lint", skipSet, onlySet), OnFail: "warn"})
		}

		addFiles := strings.Fields(files)
		gitAddArgs := append([]string{"add"}, addFiles...)
		p.Add(flow.Step{Name: fmt.Sprintf("git add %s", files), Cmd: "git", Args: gitAddArgs, Skip: shouldSkip("stage", skipSet, onlySet)})
		p.Add(flow.Step{Name: "git status", Cmd: "git", Args: []string{"status", "--short"}, Skip: shouldSkip("status", skipSet, onlySet)})

		if input.Message != "" {
			p.Add(flow.Step{Name: "git commit", Cmd: "git", Args: []string{"commit", "-m", input.Message}, Skip: shouldSkip("commit", skipSet, onlySet)})

			if input.Push {
				p.Add(flow.Step{Name: "git push", Cmd: "git", Args: []string{"push"}, Skip: shouldSkip("push", skipSet, onlySet)})
			}
		}

		runErr := p.Run()

		// Log the flow
		log := cmdlog.New("flow", dir)
		for _, r := range p.Results {
			log.Add(cmdlog.Entry{
				Cmd:      r.Step.Cmd,
				Args:     r.Step.Args,
				Dir:      r.Step.Dir,
				Status:   r.Status,
				Duration: r.Duration,
			})
		}

		logPath, _ := log.Save()

		// Build summary
		var sb strings.Builder

		for _, r := range p.Results {
			cmdLine := r.Step.Cmd
			if len(r.Step.Args) > 0 {
				cmdLine += " " + strings.Join(r.Step.Args, " ")
			}

			switch r.Status {
			case "skipped":
				fmt.Fprintf(&sb, "[skip] %s\n", cmdLine)
			default:
				fmt.Fprintf(&sb, "[%s %.1fs] %s\n", r.Status, r.Duration.Seconds(), cmdLine)
			}

			if r.Status == "failed" && r.Output != "" {
				fmt.Fprintf(&sb, "  %s\n", strings.TrimSpace(r.Output))
			}
		}

		if logPath != "" {
			fmt.Fprintf(&sb, "\nLog: %s\n", logPath)
		}

		if runErr != nil {
			fmt.Fprintf(&sb, "\nFlow failed: %s\n", runErr)
		}

		return textResult(sb.String(), runErr != nil), nil, nil
	})
}

// ── scan tool ──

type scanInput struct {
	Dir   string `json:"dir" jsonschema:"directory to scan for .scripts folders"`
	Depth int    `json:"depth,omitempty" jsonschema:"maximum directory depth (default 10)"`
	Top   int    `json:"top,omitempty" jsonschema:"number of top tools to return (default 20)"`
}

func registerScanTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scan",
		Description: "Scan directories for .scripts folders created by Claude Code sessions. Discovers projects, parses shell scripts, and ranks the most frequently used tools. Results are saved to SQLite.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input scanInput) (*mcp.CallToolResult, any, error) {
		dir := input.Dir
		if dir == "" {
			dir = "."
		}

		depth := input.Depth
		if depth == 0 {
			depth = 10
		}

		top := input.Top
		if top == 0 {
			top = 20
		}

		result, err := scanner.Scan(dir, scanner.ScanOptions{MaxDepth: depth})
		if err != nil {
			return textResult(fmt.Sprintf("Scan failed: %s", err), true), nil, nil
		}

		if len(result.Projects) == 0 {
			return textResult("No projects with .scripts found.", false), nil, nil
		}

		dbPath := cmdlog.DBPath()
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return textResult(fmt.Sprintf("Creating db directory failed: %s", err), true), nil, nil
		}

		db, err := store.Open(dbPath)
		if err != nil {
			return textResult(fmt.Sprintf("Opening database failed: %s", err), true), nil, nil
		}

		defer func() { _ = db.Close() }()

		scanID, err := db.SaveScan(dir, result)
		if err != nil {
			return textResult(fmt.Sprintf("Saving scan failed: %s", err), true), nil, nil
		}

		// Build response
		var sb strings.Builder
		fmt.Fprintf(&sb, "Scan #%d: %d projects, ", scanID, len(result.Projects))

		totalScripts, totalCommands := 0, 0
		for _, p := range result.Projects {
			totalScripts += len(p.Scripts)
			for _, s := range p.Scripts {
				totalCommands += len(s.Commands)
			}
		}

		fmt.Fprintf(&sb, "%d scripts, %d commands, %d unique tools\n\n", totalScripts, totalCommands, len(result.ToolCounts))

		limit := min(top, len(result.ToolCounts))

		fmt.Fprintf(&sb, "Top %d tools:\n", limit)

		for i := range limit {
			tc := result.ToolCounts[i]
			fmt.Fprintf(&sb, "  %2d. %-25s %d\n", i+1, tc.Name, tc.Count)
		}

		return textResult(sb.String(), false), nil, nil
	})
}

// ── stats tool ──

type statsInput struct {
	ScanID   int64 `json:"scan_id,omitempty" jsonschema:"scan ID to query (0 = latest)"`
	Top      int   `json:"top,omitempty" jsonschema:"number of top tools (default 25)"`
	Projects bool  `json:"projects,omitempty" jsonschema:"include per-project breakdown"`
	List     bool  `json:"list,omitempty" jsonschema:"list all scans"`
}

func registerStatsTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "stats",
		Description: "Query stored scan statistics from the SQLite database. Shows top tools, project breakdowns, and aggregate counts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input statsInput) (*mcp.CallToolResult, any, error) {
		dbPath := cmdlog.DBPath()
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return textResult("No database found. Run 'scan' first.", true), nil, nil
		}

		db, err := store.Open(dbPath)
		if err != nil {
			return textResult(fmt.Sprintf("Opening database failed: %s", err), true), nil, nil
		}

		defer func() { _ = db.Close() }()

		stats, err := db.GetStats()
		if err != nil {
			return textResult(fmt.Sprintf("Getting stats failed: %s", err), true), nil, nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Database: %s\n", dbPath)
		fmt.Fprintf(&sb, "Total: %d scans, %d projects, %d scripts, %d commands, %d unique tools\n\n",
			stats.TotalScans, stats.TotalProjects, stats.TotalScripts, stats.TotalCommands, stats.UniqueTools)

		if input.List {
			scans, err := db.ListScans()
			if err != nil {
				return textResult(fmt.Sprintf("Listing scans failed: %s", err), true), nil, nil
			}

			fmt.Fprintf(&sb, "Scans:\n")

			for _, sc := range scans {
				fmt.Fprintf(&sb, "  #%d  %s  %s  (%d projects, %d tools)\n",
					sc.ID, sc.ScannedAt, sc.RootDir, sc.ProjectCount, sc.ToolCount)
			}

			return textResult(sb.String(), false), nil, nil
		}

		scanID := input.ScanID
		if scanID == 0 {
			scans, err := db.ListScans()
			if err != nil || len(scans) == 0 {
				return textResult("No scans found.", false), nil, nil
			}

			scanID = scans[0].ID
		}

		top := input.Top
		if top == 0 {
			top = 25
		}

		tools, err := db.TopToolsByScan(scanID, top)
		if err != nil {
			return textResult(fmt.Sprintf("Querying tools failed: %s", err), true), nil, nil
		}

		if len(tools) > 0 {
			fmt.Fprintf(&sb, "Top %d tools (scan #%d):\n", top, scanID)

			for i, tc := range tools {
				fmt.Fprintf(&sb, "  %2d. %-25s %d\n", i+1, tc.Tool, tc.Count)
			}
		}

		if input.Projects {
			counts, err := db.CommandCountByProject(scanID)
			if err == nil {
				fmt.Fprintf(&sb, "\nProjects by command count:\n")

				for _, pc := range counts {
					fmt.Fprintf(&sb, "  %-50s %d\n", pc.ProjectPath, pc.TotalCmds)
				}
			}
		}

		return textResult(sb.String(), false), nil, nil
	})
}

// ── relations tool ──

type relationsInput struct {
	ScanID   int64 `json:"scan_id,omitempty" jsonschema:"scan ID (0 = latest)"`
	MinCount int   `json:"min_count,omitempty" jsonschema:"minimum occurrence count (default 5)"`
	Limit    int   `json:"limit,omitempty" jsonschema:"max rows per section (default 40)"`
}

func registerRelationsTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "relations",
		Description: "Analyze command co-occurrence and sequencing patterns. Shows which tools Claude Code uses together, in what order, and grouped by workflow category.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input relationsInput) (*mcp.CallToolResult, any, error) {
		dbPath := cmdlog.DBPath()
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return textResult("No database found. Run 'scan' first.", true), nil, nil
		}

		db, err := store.Open(dbPath)
		if err != nil {
			return textResult(fmt.Sprintf("Opening database failed: %s", err), true), nil, nil
		}

		defer func() { _ = db.Close() }()

		scanID := input.ScanID
		if scanID == 0 {
			scans, err := db.ListScans()
			if err != nil || len(scans) == 0 {
				return textResult("No scans found.", false), nil, nil
			}

			scanID = scans[0].ID
		}

		minCount := input.MinCount
		if minCount == 0 {
			minCount = 5
		}

		limit := input.Limit
		if limit == 0 {
			limit = 40
		}

		var sb strings.Builder

		// Clusters
		clusters, err := db.ToolClusters(scanID)
		if err == nil && len(clusters) > 0 {
			fmt.Fprintf(&sb, "Tool Clusters:\n")

			for _, cl := range clusters {
				total := 0
				for _, t := range cl.Tools {
					total += t.Count
				}

				fmt.Fprintf(&sb, "\n  %s (%d total):\n", cl.Category, total)

				for _, t := range cl.Tools {
					fmt.Fprintf(&sb, "    %-25s %d\n", t.Tool, t.Count)
				}
			}

			sb.WriteString("\n")
		}

		// Sequences
		seqs, err := db.ToolSequences(scanID, minCount, limit)
		if err == nil && len(seqs) > 0 {
			fmt.Fprintf(&sb, "Command Sequences (A → B):\n")

			for _, r := range seqs {
				fmt.Fprintf(&sb, "  %-25s → %-25s %d scripts\n", r.From, r.To, r.Count)
			}

			sb.WriteString("\n")
		}

		// Co-occurrences
		pairs, err := db.ToolCooccurrences(scanID, minCount, limit)
		if err == nil && len(pairs) > 0 {
			fmt.Fprintf(&sb, "Co-occurrence (tools in same script):\n")

			for _, p := range pairs {
				fmt.Fprintf(&sb, "  %-25s ↔ %-25s %d scripts\n", p.ToolA, p.ToolB, p.Count)
			}

			sb.WriteString("\n")
		}

		// Positions
		steps, err := db.ToolPositions(scanID, limit*3)
		if err == nil && len(steps) > 0 {
			first, middle, last := []store.WorkflowStep{}, []store.WorkflowStep{}, []store.WorkflowStep{}

			for _, ws := range steps {
				switch ws.Position {
				case "first":
					if len(first) < limit {
						first = append(first, ws)
					}
				case "middle":
					if len(middle) < limit {
						middle = append(middle, ws)
					}
				case "last":
					if len(last) < limit {
						last = append(last, ws)
					}
				}
			}

			fmt.Fprintf(&sb, "Tool Positions:\n")

			if len(first) > 0 {
				fmt.Fprintf(&sb, "  Starts with:\n")

				for _, ws := range first {
					fmt.Fprintf(&sb, "    %-25s %d\n", ws.Tool, ws.Count)
				}
			}

			if len(last) > 0 {
				fmt.Fprintf(&sb, "  Ends with:\n")

				for _, ws := range last {
					fmt.Fprintf(&sb, "    %-25s %d\n", ws.Tool, ws.Count)
				}
			}
		}

		if sb.Len() == 0 {
			return textResult("No relation data found. Run 'scan' first.", false), nil, nil
		}

		return textResult(sb.String(), false), nil, nil
	})
}

// ── patterns tool ──

type patternsInput struct {
	Action string `json:"action" jsonschema:"action: init, detect, list, suggest, rules"`
	Dir    string `json:"dir,omitempty" jsonschema:"project directory (for suggest)"`
	ScanID int64  `json:"scan_id,omitempty" jsonschema:"scan ID for detect (0 = latest)"`
}

func registerPatternsTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "patterns",
		Description: "Manage workflow patterns and rules. Actions: " +
			"'init' (create builtin patterns/rules), " +
			"'detect' (analyze scan data for new patterns), " +
			"'list' (show all patterns), " +
			"'suggest' (recommend patterns for a project dir), " +
			"'rules' (list all rules with status).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input patternsInput) (*mcp.CallToolResult, any, error) {
		ps := patterns.Default()

		switch input.Action {
		case "init":
			if err := ps.Init(); err != nil {
				return textResult(fmt.Sprintf("Init failed: %s", err), true), nil, nil
			}

			return textResult(fmt.Sprintf("Patterns initialized at %s\n  %d builtin patterns\n  %d builtin rules",
				patterns.PatternsDir(), len(patterns.BuiltinPatterns()), len(patterns.BuiltinRules())), false), nil, nil

		case "detect":
			dbPath := cmdlog.DBPath()
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				return textResult("No database found. Run 'scan' first.", true), nil, nil
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return textResult(fmt.Sprintf("Opening database failed: %s", err), true), nil, nil
			}

			defer func() { _ = db.Close() }()

			scanID := input.ScanID
			if scanID == 0 {
				scans, err := db.ListScans()
				if err != nil || len(scans) == 0 {
					return textResult("No scans found.", true), nil, nil
				}

				scanID = scans[0].ID
			}

			if err := ps.Init(); err != nil {
				return textResult(fmt.Sprintf("Init failed: %s", err), true), nil, nil
			}

			detected, err := ps.DetectPatterns(db, scanID)
			if err != nil {
				return textResult(fmt.Sprintf("Detection failed: %s", err), true), nil, nil
			}

			if len(detected) == 0 {
				return textResult("No new patterns detected.", false), nil, nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Detected %d patterns from scan #%d:\n", len(detected), scanID)

			for _, p := range detected {
				fmt.Fprintf(&sb, "  %-40s [%s] %.0f%% confidence\n", p.Name, p.Category, p.Confidence*100)
			}

			return textResult(sb.String(), false), nil, nil

		case "list":
			allPats, err := ps.LoadPatterns()
			if err != nil {
				return textResult(fmt.Sprintf("Loading patterns failed: %s", err), true), nil, nil
			}

			if len(allPats) == 0 {
				return textResult("No patterns found. Run 'patterns init' first.", false), nil, nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Patterns (%d):\n", len(allPats))

			for _, p := range allPats {
				steps := make([]string, len(p.Steps))
				for i, s := range p.Steps {
					steps[i] = s.Tool
				}

				fmt.Fprintf(&sb, "  [%s] %-40s %s\n", p.Category, p.Name, strings.Join(steps, " → "))
			}

			return textResult(sb.String(), false), nil, nil

		case "suggest":
			dir := input.Dir
			if dir == "" {
				dir = "."
			}

			suggestions, err := ps.SuggestFlows(dir)
			if err != nil {
				return textResult(fmt.Sprintf("Suggest failed: %s", err), true), nil, nil
			}

			if len(suggestions) == 0 {
				return textResult("No matching patterns for this project.", false), nil, nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Suggested workflows for %s:\n\n", dir)

			for i, p := range suggestions {
				steps := make([]string, len(p.Steps))
				for j, s := range p.Steps {
					steps[j] = s.Tool
				}

				fmt.Fprintf(&sb, "%d. %s\n   %s\n   Steps: %s\n\n", i+1, p.Name, p.Description, strings.Join(steps, " → "))
			}

			return textResult(sb.String(), false), nil, nil

		case "rules":
			rules, err := ps.LoadRules()
			if err != nil {
				return textResult(fmt.Sprintf("Loading rules failed: %s", err), true), nil, nil
			}

			if len(rules) == 0 {
				return textResult("No rules found. Run 'patterns init' first.", false), nil, nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Rules (%d):\n", len(rules))

			for _, r := range rules {
				status := "ON "
				if !r.Enabled {
					status = "OFF"
				}

				fmt.Fprintf(&sb, "  [%s] [%s] %-40s %s\n", status, r.Severity, r.Name, r.Description)
			}

			return textResult(sb.String(), false), nil, nil

		default:
			return textResult("Unknown action. Use: init, detect, list, suggest, rules", true), nil, nil
		}
	})
}

// ── scout tool ──

type scoutInput struct {
	Action string `json:"action" jsonschema:"action: navigate, screenshot, click, markdown, search, extract"`
	URL    string `json:"url,omitempty" jsonschema:"URL to navigate to or extract content from"`
	Query  string `json:"query,omitempty" jsonschema:"search query (for search action)"`
	Target string `json:"target,omitempty" jsonschema:"CSS selector (for click action)"`
}

func registerScoutTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "scout",
		Description: "Browser automation and web interaction commands. " +
			"Uses 'scout' binary (github.com/inovacc/scout) for browser automation, " +
			"falls back to 'omni curl' for HTTP requests. Auto-installs if missing. " +
			"Actions: navigate, screenshot, click, markdown, search, extract.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input scoutInput) (*mcp.CallToolResult, any, error) {
		// Ensure scout and omni are installed
		_ = deps.EnsureInstalled("scout")
		_ = deps.EnsureInstalled("omni")

		useScout := deps.IsInstalled("scout")

		switch input.Action {
		case "search":
			if input.Query == "" {
				return textResult("Error: 'query' is required for search action", true), nil, nil
			}

			if useScout {
				output, err := runCommand("scout", []string{"search", input.Query}, "")
				if err != nil {
					// Fallback to omni curl
					output, err = runCommand("omni", []string{"curl", "-s", fmt.Sprintf("https://duckduckgo.com/html/?q=%s", input.Query)}, "")
					if err != nil {
						return textResult(fmt.Sprintf("Search failed: %s", err), true), nil, nil
					}
				}

				return textResult(truncate(output, 10000), false), nil, nil
			}

			output, err := runCommand("omni", []string{"curl", "-s", fmt.Sprintf("https://duckduckgo.com/html/?q=%s", input.Query)}, "")
			if err != nil {
				return textResult(fmt.Sprintf("Search failed: %s\n%s", err, output), true), nil, nil
			}

			return textResult(truncate(output, 10000), false), nil, nil

		case "navigate":
			if input.URL == "" {
				return textResult("Error: 'url' is required for navigate action", true), nil, nil
			}

			if useScout {
				output, err := runCommand("scout", []string{"navigate", input.URL}, "")
				if err == nil {
					return textResult(truncate(output, 5000), false), nil, nil
				}
			}

			output, err := runCommand("omni", []string{"curl", "-s", input.URL}, "")
			if err != nil {
				return textResult(fmt.Sprintf("Navigate failed: %s", err), true), nil, nil
			}

			return textResult(truncate(output, 5000), false), nil, nil

		case "screenshot":
			if input.URL == "" {
				return textResult("Error: 'url' is required for screenshot action", true), nil, nil
			}

			if useScout {
				args := []string{"screenshot", input.URL}
				if input.Target != "" {
					args = append(args, "--selector", input.Target)
				}

				output, err := runCommand("scout", args, "")
				if err != nil {
					return textResult(fmt.Sprintf("Screenshot failed: %s\n%s", err, output), true), nil, nil
				}

				return textResult(output, false), nil, nil
			}

			return textResult("Screenshot requires 'scout' binary. Install: go install github.com/inovacc/scout@latest", true), nil, nil

		case "click":
			if input.Target == "" {
				return textResult("Error: 'target' (CSS selector) is required for click action", true), nil, nil
			}

			if useScout {
				args := []string{"click", input.Target}
				if input.URL != "" {
					args = append(args, "--url", input.URL)
				}

				output, err := runCommand("scout", args, "")
				if err != nil {
					return textResult(fmt.Sprintf("Click failed: %s\n%s", err, output), true), nil, nil
				}

				return textResult(output, false), nil, nil
			}

			return textResult("Click requires 'scout' binary. Install: go install github.com/inovacc/scout@latest", true), nil, nil

		case "markdown", "extract":
			if input.URL == "" {
				return textResult(fmt.Sprintf("Error: 'url' is required for %s action", input.Action), true), nil, nil
			}

			if useScout {
				output, err := runCommand("scout", []string{input.Action, input.URL}, "")
				if err == nil {
					return textResult(truncate(output, 10000), false), nil, nil
				}
			}

			output, err := runCommand("omni", []string{"curl", "-s", input.URL}, "")
			if err != nil {
				return textResult(fmt.Sprintf("Extract failed: %s", err), true), nil, nil
			}

			return textResult(truncate(output, 10000), false), nil, nil

		default:
			return textResult("Unknown action. Use: navigate, screenshot, click, markdown, search, extract", true), nil, nil
		}
	})
}

// ── next-steps tool ──

type nextStepsInput struct {
	Dir   string `json:"dir" jsonschema:"project directory to analyze"`
	After string `json:"after,omitempty" jsonschema:"what just happened: push, commit, merge, release, deploy, test, refactor"`
}

func registerNextStepsTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "next-steps",
		Description: "Suggest what to do next after completing an action. " +
			"Analyzes project state and recommends follow-up actions based on patterns. " +
			"Especially useful after: push, commit, merge, release, deploy, test, refactor.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input nextStepsInput) (*mcp.CallToolResult, any, error) {
		dir := input.Dir
		if dir == "" {
			var err error

			dir, err = os.Getwd()
			if err != nil {
				return textResult("Error: cannot determine working directory", true), nil, nil //nolint:nilerr // intentional: return error as MCP content
			}
		}

		after := input.After
		if after == "" {
			after = "push"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Suggested next steps after '%s' in %s:\n\n", after, dir)

		suggestions := suggestNextSteps(after, dir)
		for i, s := range suggestions {
			fmt.Fprintf(&sb, "  %d. %s\n     %s\n", i+1, s.title, s.detail)

			if s.command != "" {
				fmt.Fprintf(&sb, "     $ %s\n", s.command)
			}

			sb.WriteString("\n")
		}

		// Also check for pattern suggestions
		ps := patterns.Default()

		patSuggestions, err := ps.SuggestFlows(dir)
		if err == nil && len(patSuggestions) > 0 {
			fmt.Fprintf(&sb, "Available workflows for this project:\n")

			shown := 0

			for _, p := range patSuggestions {
				if p.Category == "flow" || p.Category == "deploy" {
					steps := make([]string, len(p.Steps))
					for i, s := range p.Steps {
						steps[i] = s.Tool
					}

					fmt.Fprintf(&sb, "  - %s: %s\n", p.Name, strings.Join(steps, " → "))

					shown++
					if shown >= 3 {
						break
					}
				}
			}
		}

		return textResult(sb.String(), false), nil, nil
	})
}

type suggestion struct {
	title   string
	detail  string
	command string
}

func suggestNextSteps(after string, dir string) []suggestion {
	hasGoMod := fileExists(filepath.Join(dir, "go.mod"))
	_ = dirExists(filepath.Join(dir, ".git"))
	hasGoreleaser := fileExists(filepath.Join(dir, ".goreleaser.yaml")) || fileExists(filepath.Join(dir, ".goreleaser.yml"))
	hasWorkflows := dirExists(filepath.Join(dir, ".github", "workflows"))
	hasDockerfile := fileExists(filepath.Join(dir, "Dockerfile"))

	switch after {
	case "push":
		s := []suggestion{
			{title: "Check CI status", detail: "Verify your push triggered CI and it's passing", command: "gh run list --limit 3"},
			{title: "Create or update pull request", detail: "If working on a feature branch, open a PR for review", command: "gh pr create --fill"},
		}
		if hasWorkflows {
			s = append(s, suggestion{title: "Monitor workflow runs", detail: "Watch for build/test failures in GitHub Actions", command: "gh run watch"})
		}

		if hasGoreleaser {
			s = append(s, suggestion{title: "Consider a release", detail: "If this is a milestone, tag and release", command: "git tag v0.x.0 && git push --tags"})
		}

		s = append(s, suggestion{title: "Update documentation", detail: "If you added features, update README.md and docs/"})
		s = append(s, suggestion{title: "Open follow-up issues", detail: "Track any TODOs or improvements identified during development", command: "gh issue create"})

		return s

	case "commit":
		s := []suggestion{
			{title: "Push to remote", detail: "Share your changes with the team", command: "git push"},
		}
		if hasGoMod {
			s = append(s, suggestion{title: "Run full test suite", detail: "Verify nothing is broken", command: "go test -race ./..."})
		}

		s = append(s, suggestion{title: "Review changes", detail: "Double-check your commit with a diff", command: "git log --oneline -5 && git diff HEAD~1"})

		return s

	case "merge":
		s := []suggestion{
			{title: "Delete merged branch", detail: "Clean up the feature branch", command: "git branch -d feature/xxx"},
			{title: "Pull latest main", detail: "Ensure your local main is up to date", command: "git checkout main && git pull"},
		}
		if hasGoMod {
			s = append(s, suggestion{title: "Run tests on main", detail: "Verify the merge didn't break anything", command: "go test ./..."})
		}

		s = append(s, suggestion{title: "Check for dependent PRs", detail: "Unblock any PRs that depended on this merge", command: "gh pr list"})

		return s

	case "release":
		s := []suggestion{
			{title: "Verify release artifacts", detail: "Check that binaries were built correctly", command: "gh release view --json assets"},
		}
		s = append(s, suggestion{title: "Update changelog", detail: "Document what changed in this release"})

		s = append(s, suggestion{title: "Announce the release", detail: "Notify users/team about the new version"})
		if hasDockerfile {
			s = append(s, suggestion{title: "Update container images", detail: "Build and push new Docker images with the release tag"})
		}

		s = append(s, suggestion{title: "Bump version for next cycle", detail: "Start working on the next version"})

		return s

	case "deploy":
		s := []suggestion{
			{title: "Monitor health checks", detail: "Verify the deployment is healthy", command: "kubectl get pods"},
			{title: "Check logs", detail: "Watch for errors in the deployed application"},
			{title: "Run smoke tests", detail: "Verify critical paths are working"},
			{title: "Update deployment docs", detail: "Record what was deployed and any configuration changes"},
		}

		return s

	case "test":
		s := []suggestion{
			{title: "Check coverage", detail: "Identify untested code paths", command: "go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out"},
		}
		if hasGoMod {
			s = append(s, suggestion{title: "Run benchmarks", detail: "Check for performance regressions", command: "go test -bench=. ./..."})
			s = append(s, suggestion{title: "Run race detector", detail: "Check for data races", command: "go test -race ./..."})
		}

		s = append(s, suggestion{title: "Commit passing tests", detail: "Stage and commit if tests are green", command: "git add . && git commit -m 'test: add tests'"})

		return s

	case "refactor":
		s := make([]suggestion, 0, 4)
		s = append(s, suggestion{title: "Run full test suite", detail: "Ensure refactoring didn't break behavior", command: "go test ./..."})
		s = append(s, suggestion{title: "Run linter", detail: "Check for new issues introduced by refactoring", command: "golangci-lint run ./..."})
		s = append(s, suggestion{title: "Review diff carefully", detail: "Verify the refactoring is correct", command: "git diff"})
		s = append(s, suggestion{title: "Commit with clear message", detail: "Explain why the refactoring was needed"})

		return s

	default:
		return []suggestion{
			{title: "Check project status", detail: "Review current state", command: "git status && git log --oneline -5"},
			{title: "Run tests", detail: "Verify everything works", command: "go test ./..."},
			{title: "Check open issues", detail: "Find the next thing to work on", command: "gh issue list --limit 5"},
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... (truncated)"
	}

	return s
}

func runCommand(cmd string, args []string, dir string) (string, error) {
	c := exec.Command(cmd, args...)
	if dir != "" {
		c.Dir = dir
	}

	c.Env = os.Environ()
	out, err := c.CombinedOutput()

	return string(out), err
}

// ── helpers ──

func textResult(text string, isError bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: isError,
	}
}

func shouldSkip(name string, skipSet, onlySet map[string]bool) bool {
	if len(onlySet) > 0 {
		return !onlySet[name]
	}

	return skipSet[name]
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool)

	for _, item := range items {
		for part := range strings.SplitSeq(item, ",") {
			set[strings.TrimSpace(part)] = true
		}
	}

	return set
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ── install support ──

// ClaudeConfig represents the structure of Claude Code's settings file.
type ClaudeConfig struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

// MCPServerConfig is the config for a single MCP server.
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// InstallGlobal writes the MCP server config to Claude Code's global settings.
func InstallGlobal(binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configPath := filepath.Join(home, ".claude.json")

	// Read existing config
	config := make(map[string]json.RawMessage)

	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parsing %s: %w", configPath, err)
		}
	}

	// Get or create mcpServers
	mcpServers := make(map[string]json.RawMessage)
	if raw, ok := config["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &mcpServers); err != nil {
			return fmt.Errorf("parsing mcpServers: %w", err)
		}
	}

	// Add repited server
	serverConfig := MCPServerConfig{
		Command: binaryPath,
		Args:    []string{"mcp", "serve"},
	}

	serverJSON, err := json.Marshal(serverConfig)
	if err != nil {
		return fmt.Errorf("marshaling server config: %w", err)
	}

	mcpServers["repited"] = serverJSON

	// Write back
	serversJSON, err := json.Marshal(mcpServers)
	if err != nil {
		return fmt.Errorf("marshaling mcpServers: %w", err)
	}

	config["mcpServers"] = serversJSON

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}

	return nil
}

// InstallProject writes the MCP server config to a project-level .mcp.json file.
func InstallProject(binaryPath, projectDir string) error {
	configPath := filepath.Join(projectDir, ".mcp.json")

	// Read existing config
	config := make(map[string]json.RawMessage)

	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parsing %s: %w", configPath, err)
		}
	}

	// Get or create mcpServers
	mcpServers := make(map[string]json.RawMessage)

	if raw, ok := config["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &mcpServers); err != nil {
			return fmt.Errorf("parsing mcpServers: %w", err)
		}
	}

	// Add repited server
	serverConfig := MCPServerConfig{
		Command: binaryPath,
		Args:    []string{"mcp", "serve"},
	}

	serverJSON, err := json.Marshal(serverConfig)
	if err != nil {
		return fmt.Errorf("marshaling server config: %w", err)
	}

	mcpServers["repited"] = serverJSON

	// Write back
	serversJSON, err := json.Marshal(mcpServers)
	if err != nil {
		return fmt.Errorf("marshaling mcpServers: %w", err)
	}

	config["mcpServers"] = serversJSON

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}

	return nil
}

// UninstallGlobal removes the MCP server config from Claude Code's global settings.
func UninstallGlobal() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configPath := filepath.Join(home, ".claude.json")

	config := make(map[string]json.RawMessage)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil //nolint:nilerr // intentional: no config file means nothing to uninstall
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parsing %s: %w", configPath, err)
	}

	raw, ok := config["mcpServers"]
	if !ok {
		return nil
	}

	mcpServers := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &mcpServers); err != nil {
		return fmt.Errorf("parsing mcpServers: %w", err)
	}

	delete(mcpServers, "repited")

	serversJSON, err := json.Marshal(mcpServers)
	if err != nil {
		return fmt.Errorf("marshaling mcpServers: %w", err)
	}

	config["mcpServers"] = serversJSON

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(configPath, output, 0o644)
}
