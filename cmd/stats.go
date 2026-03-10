package cmd

import (
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
}

func runStats(cmd *cobra.Command, _ []string) error {
	dbPath, _ := cmd.Flags().GetString("db")
	scanID, _ := cmd.Flags().GetInt64("scan")
	top, _ := cmd.Flags().GetInt("top")
	showProjects, _ := cmd.Flags().GetBool("projects")
	listScans, _ := cmd.Flags().GetBool("list")

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

	// Top tools
	tools, err := db.TopToolsByScan(scanID, top)
	if err != nil {
		return fmt.Errorf("querying top tools: %w", err)
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
