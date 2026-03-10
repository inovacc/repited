package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/inovacc/repited/internal/patterns"
	"github.com/spf13/cobra"
)

var patternsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export patterns to a JSON file for sharing",
	Long: `Export workflow patterns to a portable JSON file.

By default, only user-defined patterns are exported. Use flags to include
builtin or detected patterns.

Examples:
  repited patterns export                          # user patterns to stdout
  repited patterns export --file team-patterns.json
  repited patterns export --all --file backup.json
  repited patterns export --builtin --user --file shared.json`,
	RunE: runPatternsExport,
}

var patternsImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import patterns from a JSON file",
	Long: `Import workflow patterns from a file exported by 'patterns export'.

Imported patterns are saved into user-patterns.json (builtins are never modified).

Conflict resolution:
  (default)     Skip patterns that already exist (by name)
  --merge       Merge tools from imported pattern into existing one
  --overwrite   Replace existing patterns with imported ones

Examples:
  repited patterns import team-patterns.json
  repited patterns import shared.json --merge
  repited patterns import backup.json --overwrite`,
	Args: cobra.ExactArgs(1),
	RunE: runPatternsImport,
}

func init() {
	patternsCmd.AddCommand(patternsExportCmd)
	patternsCmd.AddCommand(patternsImportCmd)

	patternsExportCmd.Flags().StringP("file", "f", "", "output file path (default: stdout)")
	patternsExportCmd.Flags().Bool("builtin", false, "include builtin patterns")
	patternsExportCmd.Flags().Bool("user", false, "include user-defined patterns (default when no flags)")
	patternsExportCmd.Flags().Bool("all", false, "include all patterns (builtin + user + detected)")

	patternsImportCmd.Flags().Bool("merge", false, "merge tools from imported patterns into existing ones")
	patternsImportCmd.Flags().Bool("overwrite", false, "replace existing patterns with imported ones")
}

func runPatternsExport(cmd *cobra.Command, _ []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	inclBuiltin, _ := cmd.Flags().GetBool("builtin")
	inclUser, _ := cmd.Flags().GetBool("user")
	inclAll, _ := cmd.Flags().GetBool("all")

	if inclAll {
		inclBuiltin = true
		inclUser = true
	}

	// Default: export user patterns when no flags specified
	if !inclBuiltin && !inclUser && !inclAll {
		inclUser = true
	}

	ps := patterns.Default()

	exportData, err := ps.ExportPatterns(inclBuiltin, inclUser, inclAll)
	if err != nil {
		return fmt.Errorf("exporting patterns: %w", err)
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling export data: %w", err)
	}

	if filePath == "" {
		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("writing export file: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Exported %d patterns to %s\n", len(exportData.Patterns), filePath)

	return nil
}

func runPatternsImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	doMerge, _ := cmd.Flags().GetBool("merge")
	doOverwrite, _ := cmd.Flags().GetBool("overwrite")

	if doMerge && doOverwrite {
		return fmt.Errorf("--merge and --overwrite are mutually exclusive")
	}

	mode := "skip"
	if doMerge {
		mode = "merge"
	} else if doOverwrite {
		mode = "overwrite"
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	var exportData patterns.ExportData

	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("parsing import file: %w", err)
	}

	if len(exportData.Patterns) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No patterns found in import file.")

		return nil
	}

	ps := patterns.Default()

	imported, skippedCount, mergedCount, err := ps.ImportPatterns(&exportData, mode)
	if err != nil {
		return fmt.Errorf("importing patterns: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Import summary: %d imported, %d skipped, %d %s\n",
		imported, skippedCount, mergedCount, mode+"d")
	_, _ = fmt.Fprintf(os.Stdout, "Patterns saved to %s\n", ps.UserPatternsFile())

	return nil
}
