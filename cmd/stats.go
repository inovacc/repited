package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/inovacc/repited/internal/store"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show statistics from the SQLite database",
	Long: `Query the SQLite database to show stored scan results,
top tools, project breakdowns, and aggregate statistics.`,
	RunE: runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)

	statsCmd.Flags().StringP("db", "", defaultDBPath(), "SQLite database path")
	statsCmd.Flags().Int64P("scan", "s", 0, "show details for a specific scan ID (0 = latest)")
	statsCmd.Flags().IntP("top", "t", 25, "number of top tools to display")
	statsCmd.Flags().BoolP("projects", "p", false, "show per-project command counts")
	statsCmd.Flags().BoolP("list", "l", false, "list all scans")
	statsCmd.Flags().StringP("filter", "f", "", "filter tools by name (case-insensitive substring match)")
	statsCmd.Flags().Bool("json", false, "output results as JSON")
}

func runStats(cmd *cobra.Command, _ []string) error {
	dbPath, _ := cmd.Flags().GetString("db")
	scanID, _ := cmd.Flags().GetInt64("scan")
	top, _ := cmd.Flags().GetInt("top")
	showProjects, _ := cmd.Flags().GetBool("projects")
	listScans, _ := cmd.Flags().GetBool("list")
	filter, _ := cmd.Flags().GetString("filter")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database not found at %s — run 'repited scan' first", dbPath)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	defer func() { _ = db.Close() }()

	// Show aggregate stats
	stats, err := db.GetStats()
	if err != nil {
		return fmt.Errorf("getting stats: %w", err)
	}

	if jsonOutput {
		return printStatsJSON(db, stats, scanID, top, filter)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Database: %s\n", dbPath)
	_, _ = fmt.Fprintf(os.Stdout, "Total scans: %d | Projects: %d | Scripts: %d | Commands: %d | Unique tools: %d\n\n",
		stats.TotalScans, stats.TotalProjects, stats.TotalScripts, stats.TotalCommands, stats.UniqueTools)

	if listScans {
		return printScanList(db)
	}

	// Resolve scan ID
	if scanID == 0 {
		scans, err := db.ListScans()
		if err != nil {
			return fmt.Errorf("listing scans: %w", err)
		}

		if len(scans) == 0 {
			_, _ = fmt.Fprintln(os.Stdout, "No scans found.")
			return nil
		}

		scanID = scans[0].ID
	}

	if filter != "" {
		_, _ = fmt.Fprintf(os.Stdout, "Filtered by: %s\n\n", filter)
	}

	// Top tools
	tools, err := db.TopToolsByScan(scanID, top)
	if err != nil {
		return fmt.Errorf("querying top tools: %w", err)
	}

	if filter != "" {
		tools = filterToolCounts(tools, filter)
	}

	if len(tools) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "Top %d tools (scan #%d):\n\n", top, scanID)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "RANK\tTOOL\tCOUNT\tBAR")

		maxCount := tools[0].Count
		for i, tc := range tools {
			barLen := max((tc.Count*30)/maxCount, 1)

			bar := strings.Repeat("█", barLen)
			_, _ = fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", i+1, tc.Tool, tc.Count, bar)
		}

		_ = w.Flush()

		_, _ = fmt.Fprintln(os.Stdout)
	}

	// Per-project breakdown
	if showProjects {
		counts, err := db.CommandCountByProject(scanID)
		if err != nil {
			return fmt.Errorf("querying project counts: %w", err)
		}

		_, _ = fmt.Fprintf(os.Stdout, "Projects by command count (scan #%d):\n\n", scanID)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "PROJECT\tCOMMANDS")

		for _, pc := range counts {
			_, _ = fmt.Fprintf(w, "%s\t%d\n", pc.ProjectPath, pc.TotalCmds)
		}

		_ = w.Flush()
	}

	return nil
}

type statsJSONOutput struct {
	Stats         statsJSONStats         `json:"stats"`
	Scans         []statsJSONScan        `json:"scans"`
	TopTools      []statsJSONToolCount   `json:"top_tools"`
	ProjectCounts []statsJSONProjectCount `json:"project_counts"`
}

type statsJSONStats struct {
	TotalScans    int `json:"total_scans"`
	TotalProjects int `json:"total_projects"`
	TotalScripts  int `json:"total_scripts"`
	TotalCommands int `json:"total_commands"`
	UniqueTools   int `json:"unique_tools"`
}

type statsJSONScan struct {
	ID           int64  `json:"id"`
	RootDir      string `json:"root_dir"`
	ScannedAt    string `json:"scanned_at"`
	ProjectCount int    `json:"project_count"`
	ToolCount    int    `json:"tool_count"`
}

type statsJSONToolCount struct {
	Tool  string `json:"tool"`
	Count int    `json:"count"`
}

type statsJSONProjectCount struct {
	ProjectPath string `json:"project_path"`
	TotalCmds   int    `json:"total_cmds"`
}

func printStatsJSON(db *store.Store, stats *store.Stats, scanID int64, top int, filter string) error {
	// Build scans list
	scanSummaries, err := db.ListScans()
	if err != nil {
		return fmt.Errorf("listing scans: %w", err)
	}

	scans := make([]statsJSONScan, 0, len(scanSummaries))

	for _, sc := range scanSummaries {
		scans = append(scans, statsJSONScan{
			ID:           sc.ID,
			RootDir:      sc.RootDir,
			ScannedAt:    sc.ScannedAt,
			ProjectCount: sc.ProjectCount,
			ToolCount:    sc.ToolCount,
		})
	}

	// Resolve scan ID for tools/projects
	if scanID == 0 && len(scanSummaries) > 0 {
		scanID = scanSummaries[0].ID
	}

	// Top tools
	var topTools []statsJSONToolCount

	if scanID > 0 {
		tools, err := db.TopToolsByScan(scanID, top)
		if err != nil {
			return fmt.Errorf("querying top tools: %w", err)
		}

		if filter != "" {
			tools = filterToolCounts(tools, filter)
		}

		topTools = make([]statsJSONToolCount, 0, len(tools))

		for _, tc := range tools {
			topTools = append(topTools, statsJSONToolCount{
				Tool:  tc.Tool,
				Count: tc.Count,
			})
		}
	}

	// Project counts
	var projectCounts []statsJSONProjectCount

	if scanID > 0 {
		counts, err := db.CommandCountByProject(scanID)
		if err != nil {
			return fmt.Errorf("querying project counts: %w", err)
		}

		projectCounts = make([]statsJSONProjectCount, 0, len(counts))

		for _, pc := range counts {
			projectCounts = append(projectCounts, statsJSONProjectCount{
				ProjectPath: pc.ProjectPath,
				TotalCmds:   pc.TotalCmds,
			})
		}
	}

	out := statsJSONOutput{
		Stats: statsJSONStats{
			TotalScans:    stats.TotalScans,
			TotalProjects: stats.TotalProjects,
			TotalScripts:  stats.TotalScripts,
			TotalCommands: stats.TotalCommands,
			UniqueTools:   stats.UniqueTools,
		},
		Scans:         scans,
		TopTools:      topTools,
		ProjectCounts: projectCounts,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, string(data))

	return nil
}

func printScanList(db *store.Store) error {
	scans, err := db.ListScans()
	if err != nil {
		return fmt.Errorf("listing scans: %w", err)
	}

	if len(scans) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No scans found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tROOT DIR\tSCANNED AT\tPROJECTS\tUNIQUE TOOLS")

	for _, sc := range scans {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%d\n",
			sc.ID, sc.RootDir, sc.ScannedAt, sc.ProjectCount, sc.ToolCount)
	}

	_ = w.Flush()

	return nil
}
