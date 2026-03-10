package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/inovacc/repited/internal/cmdlog"
	"github.com/inovacc/repited/internal/scanner"
	"github.com/inovacc/repited/internal/store"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan [directory]",
	Short: "Scan directories for .scripts folders and analyze tool usage",
	Long: `Walk through directories to find projects containing .git and .scripts folders.
Reads all script files inside .scripts and ranks the most frequently called tools and commands.
Results are saved to a SQLite database for later analysis.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().IntP("depth", "d", 10, "maximum directory depth to scan")
	scanCmd.Flags().IntP("top", "t", 20, "number of top tools to display")
	scanCmd.Flags().BoolP("projects", "p", false, "show per-project breakdown")
	scanCmd.Flags().StringP("db", "", defaultDBPath(), "SQLite database path")
	scanCmd.Flags().StringSlice("exclude", nil, "Additional directory names to skip during scan")
	scanCmd.Flags().Bool("json", false, "output results as JSON")
	scanCmd.Flags().BoolP("watch", "w", false, "watch .scripts directories and re-scan on changes")
}

func defaultDBPath() string {
	return cmdlog.DBPath()
}

func runScan(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	depth, _ := cmd.Flags().GetInt("depth")
	top, _ := cmd.Flags().GetInt("top")
	showProjects, _ := cmd.Flags().GetBool("projects")
	dbPath, _ := cmd.Flags().GetString("db")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	watch, _ := cmd.Flags().GetBool("watch")

	opts := scanner.ScanOptions{
		MaxDepth: depth,
		Exclude:  exclude,
	}

	result, err := executeScan(dir, depth, top, showProjects, dbPath, jsonOutput, opts)
	if err != nil {
		return err
	}

	if !watch || len(result.Projects) == 0 {
		return nil
	}

	return runWatchLoop(cmd.Context(), dir, depth, top, showProjects, dbPath, jsonOutput, opts, result)
}

// executeScan performs a single scan and prints the results.
func executeScan(dir string, depth, top int, showProjects bool, dbPath string, jsonOutput bool, opts scanner.ScanOptions) (*scanner.ScanResult, error) {
	_, _ = fmt.Fprintf(os.Stdout, "Scanning %s (depth=%d)...\n", dir, depth)

	result, err := scanner.Scan(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	if len(result.Projects) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No projects with .scripts found.")

		return result, nil
	}

	// Ensure database directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	// Save to SQLite
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	defer func() { _ = db.Close() }()

	scanID, err := db.SaveScan(dir, result)
	if err != nil {
		return nil, fmt.Errorf("saving scan: %w", err)
	}

	if jsonOutput {
		return result, printScanJSON(result, scanID, dbPath)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Found %d project(s) with .scripts — saved to %s (scan #%d)\n\n", len(result.Projects), dbPath, scanID)

	if showProjects {
		printProjects(result.Projects)
	}

	printToolRanking(result.ToolCounts, top)

	// Print summary stats
	totalScripts := 0
	totalCommands := 0

	for _, p := range result.Projects {
		totalScripts += len(p.Scripts)

		for _, s := range p.Scripts {
			totalCommands += len(s.Commands)
		}
	}

	_, _ = fmt.Fprintf(os.Stdout, "\nSummary: %d projects, %d scripts, %d commands, %d unique tools\n",
		len(result.Projects), totalScripts, totalCommands, len(result.ToolCounts))

	// Log this invocation
	lg := cmdlog.New("scan", dir)
	lg.Add(cmdlog.Entry{
		Cmd:    "repited",
		Args:   []string{"scan", dir, "--depth", fmt.Sprintf("%d", depth), "--top", fmt.Sprintf("%d", top)},
		Dir:    dir,
		Status: "ok",
	})

	if logPath, err := lg.Save(); err == nil {
		_, _ = fmt.Fprintf(os.Stdout, "Log: %s\n", logPath)
	}

	return result, nil
}

// runWatchLoop sets up filesystem watchers on .scripts directories and re-scans on changes.
func runWatchLoop(parent context.Context, dir string, depth, top int, showProjects bool, dbPath string, jsonOutput bool, opts scanner.ScanOptions, result *scanner.ScanResult) error {
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt)
	defer cancel()

	scriptsDirs := collectScriptsDirs(result)
	scanCount := 1

	// Create watcher first with a nil callback; set the real callback after
	// so the closure can reference w itself for adding new directories.
	w, err := scanner.NewWatcher(scriptsDirs, nil)
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}

	defer func() { _ = w.Close() }()

	w.SetOnChange(func() {
		scanCount++
		_, _ = fmt.Fprintf(os.Stdout, "\n%s\n", strings.Repeat("-", 60))
		_, _ = fmt.Fprintf(os.Stdout, "[%s] Re-scanning (#%d)...\n", time.Now().Format(time.RFC3339), scanCount)

		newResult, scanErr := executeScan(dir, depth, top, showProjects, dbPath, jsonOutput, opts)
		if scanErr != nil {
			slog.Error("re-scan failed", "err", scanErr)

			return
		}

		if len(newResult.Projects) == 0 {
			return
		}

		// Watch any new .scripts directories that appeared
		for _, d := range collectScriptsDirs(newResult) {
			if addErr := w.AddDir(d); addErr != nil {
				slog.Error("failed to watch new directory", "dir", d, "err", addErr)
			}
		}
	})

	slog.Info("watch mode active", "dirs", len(scriptsDirs))
	_, _ = fmt.Fprintf(os.Stdout, "\nWatching %d .scripts director(ies) for changes. Press Ctrl+C to stop.\n", len(scriptsDirs))

	if startErr := w.Start(ctx); startErr != nil && ctx.Err() != nil {
		_, _ = fmt.Fprintf(os.Stdout, "\nStopped watching. Total scans: %d\n", scanCount)

		return nil
	}

	return nil
}

// collectScriptsDirs extracts the .scripts directory paths from a scan result.
func collectScriptsDirs(result *scanner.ScanResult) []string {
	dirs := make([]string, 0, len(result.Projects))

	for _, p := range result.Projects {
		scriptsDir := filepath.Join(p.Path, ".scripts")
		dirs = append(dirs, scriptsDir)
	}

	return dirs
}

type scanJSONOutput struct {
	Projects   []scanJSONProject   `json:"projects"`
	ToolCounts []scanJSONToolCount `json:"tool_counts"`
	ScanID     int64               `json:"scan_id"`
	DBPath     string              `json:"db_path"`
}

type scanJSONProject struct {
	Path    string           `json:"path"`
	Scripts []scanJSONScript `json:"scripts"`
}

type scanJSONScript struct {
	Name     string   `json:"name"`
	Commands []string `json:"commands"`
}

type scanJSONToolCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func printScanJSON(result *scanner.ScanResult, scanID int64, dbPath string) error {
	projects := make([]scanJSONProject, 0, len(result.Projects))

	for _, p := range result.Projects {
		scripts := make([]scanJSONScript, 0, len(p.Scripts))

		for _, s := range p.Scripts {
			scripts = append(scripts, scanJSONScript{
				Name:     s.Name,
				Commands: s.Commands,
			})
		}

		projects = append(projects, scanJSONProject{
			Path:    p.Path,
			Scripts: scripts,
		})
	}

	toolCounts := make([]scanJSONToolCount, 0, len(result.ToolCounts))

	for _, tc := range result.ToolCounts {
		toolCounts = append(toolCounts, scanJSONToolCount{
			Name:  tc.Name,
			Count: tc.Count,
		})
	}

	out := scanJSONOutput{
		Projects:   projects,
		ToolCounts: toolCounts,
		ScanID:     scanID,
		DBPath:     dbPath,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, string(data))

	return nil
}

func printProjects(projects []scanner.Project) {
	for _, p := range projects {
		_, _ = fmt.Fprintf(os.Stdout, "  %s (%d scripts)\n", p.Path, len(p.Scripts))

		for _, s := range p.Scripts {
			_, _ = fmt.Fprintf(os.Stdout, "    - %s\n", s.Name)
		}
	}

	_, _ = fmt.Fprintln(os.Stdout)
}

func printToolRanking(counts []scanner.ToolCount, top int) {
	if len(counts) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No commands found in scripts.")
		return
	}

	limit := min(top, len(counts))

	_, _ = fmt.Fprintf(os.Stdout, "Top %d most used tools/commands:\n\n", limit)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "RANK\tTOOL\tCOUNT\tBAR")

	maxCount := counts[0].Count
	for i := range limit {
		tc := counts[i]

		barLen := max((tc.Count*30)/maxCount, 1)

		bar := strings.Repeat("█", barLen)
		_, _ = fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", i+1, tc.Name, tc.Count, bar)
	}

	_ = w.Flush()
}
