package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inovacc/repited/internal/cmdlog"
	"github.com/inovacc/repited/internal/patterns"
	"github.com/inovacc/repited/internal/store"
	"github.com/spf13/cobra"
)

var patternsCmd = &cobra.Command{
	Use:   "patterns",
	Short: "Manage workflow patterns and rules",
	Long: `Discover, list, and manage workflow patterns detected from scan data.

Patterns are stored in C:\Users\<user>\AppData\Local\Repited\patterns\ and include:
  - Builtin patterns (Go, Node.js, Rust, Docker, K8s, Terraform flows)
  - Detected patterns from scan analysis (sequential flows, guard pairs, clusters)
  - Rules for pre-commit checks, quality gates, and conventions`,
}

var patternsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize patterns directory with builtin patterns and rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		ps := patterns.Default()
		if err := ps.Init(); err != nil {
			return fmt.Errorf("initializing patterns: %w", err)
		}

		dir := patterns.PatternsDir()
		_, _ = fmt.Fprintf(os.Stdout, "Patterns initialized at %s\n", dir)
		_, _ = fmt.Fprintf(os.Stdout, "  builtin-patterns.json  — %d workflow patterns\n", len(patterns.BuiltinPatterns()))
		_, _ = fmt.Fprintf(os.Stdout, "  builtin-rules.json     — %d rules\n", len(patterns.BuiltinRules()))

		return nil
	},
}

var patternsDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Analyze scan data and detect new workflow patterns",
	Long: `Uses AI-informed heuristics to discover workflow patterns from stored scan data:
  - Sequential flows (chains of commands that commonly run together)
  - Guard patterns (prerequisites that always precede other commands)
  - Teardown patterns (commands that commonly end scripts)
  - Cluster patterns (tools grouped by category with high co-occurrence)`,
	RunE: runPatternsDetect,
}

var patternsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all known patterns and rules",
	RunE:  runPatternsList,
}

var patternsSuggestCmd = &cobra.Command{
	Use:   "suggest [directory]",
	Short: "Suggest applicable patterns for a project directory",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPatternsSuggest,
}

var patternsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a user-defined workflow pattern",
	Long: `Create a custom workflow pattern with specified tools and category.

Example:
  repited patterns create "my-deploy" --tools "go build,go test,docker build" --category deploy --description "My deploy flow"`,
	Args: cobra.ExactArgs(1),
	RunE: runPatternsCreate,
}

var patternsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a user-defined pattern (cannot delete builtins)",
	Args:  cobra.ExactArgs(1),
	RunE:  runPatternsDelete,
}

var patternsRulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "List all rules with their status",
	RunE:  runPatternsRules,
}

func init() {
	rootCmd.AddCommand(patternsCmd)
	patternsCmd.AddCommand(patternsInitCmd)
	patternsCmd.AddCommand(patternsDetectCmd)
	patternsCmd.AddCommand(patternsListCmd)
	patternsCmd.AddCommand(patternsSuggestCmd)
	patternsCmd.AddCommand(patternsRulesCmd)
	patternsCmd.AddCommand(patternsCreateCmd)
	patternsCmd.AddCommand(patternsDeleteCmd)

	patternsDetectCmd.Flags().Int64P("scan", "s", 0, "scan ID (0 = latest)")
	patternsDetectCmd.Flags().StringP("db", "", defaultDBPath(), "SQLite database path")

	patternsListCmd.Flags().BoolP("json", "j", false, "output as JSON")
	patternsListCmd.Flags().StringP("category", "c", "", "filter by category (flow, guard, deploy, test, setup, refactor)")

	patternsSuggestCmd.Flags().BoolP("json", "j", false, "output as JSON")

	patternsCreateCmd.Flags().StringP("tools", "t", "", "comma-separated list of tools/commands (required)")
	patternsCreateCmd.Flags().StringP("category", "c", "flow", "pattern category (flow, guard, deploy, test, setup, refactor)")
	patternsCreateCmd.Flags().StringP("description", "d", "", "pattern description")

	_ = patternsCreateCmd.MarkFlagRequired("tools")
}

func runPatternsDetect(cmd *cobra.Command, _ []string) error {
	dbPath, _ := cmd.Flags().GetString("db")
	scanID, _ := cmd.Flags().GetInt64("scan")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database not found at %s — run 'repited scan' first", dbPath)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	defer func() { _ = db.Close() }()

	if scanID == 0 {
		scans, err := db.ListScans()
		if err != nil || len(scans) == 0 {
			return fmt.Errorf("no scans found — run 'repited scan' first")
		}

		scanID = scans[0].ID
	}

	// Ensure patterns dir exists with builtins
	ps := patterns.Default()
	if err := ps.Init(); err != nil {
		return fmt.Errorf("initializing patterns: %w", err)
	}

	detected, err := ps.DetectPatterns(db, scanID)
	if err != nil {
		return fmt.Errorf("detecting patterns: %w", err)
	}

	if len(detected) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No new patterns detected from scan data.")
		_, _ = fmt.Fprintln(os.Stdout, "Try scanning more directories with 'repited scan' first.")

		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "Detected %d new patterns from scan #%d:\n\n", len(detected), scanID)

	for _, p := range detected {
		icon := categoryIcon(p.Category)
		_, _ = fmt.Fprintf(os.Stdout, "  %s %-40s [%s] (%.0f%% confidence, %d occurrences)\n",
			icon, p.Name, p.Category, p.Confidence*100, p.Occurrences)

		if len(p.Steps) > 0 {
			stepNames := make([]string, len(p.Steps))
			for i, s := range p.Steps {
				stepNames[i] = s.Tool
			}

			_, _ = fmt.Fprintf(os.Stdout, "    %s\n", strings.Join(stepNames, " → "))
		}
	}

	_, _ = fmt.Fprintf(os.Stdout, "\nPatterns saved to %s\n", patterns.PatternsDir())

	return nil
}

func runPatternsList(cmd *cobra.Command, _ []string) error {
	asJSON, _ := cmd.Flags().GetBool("json")
	category, _ := cmd.Flags().GetString("category")

	ps := patterns.Default()

	allPatterns, err := ps.LoadPatterns()
	if err != nil {
		return fmt.Errorf("loading patterns: %w", err)
	}

	if category != "" {
		var filtered []patterns.Pattern

		for _, p := range allPatterns {
			if p.Category == category {
				filtered = append(filtered, p)
			}
		}

		allPatterns = filtered
	}

	if asJSON {
		data, err := json.MarshalIndent(allPatterns, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling patterns: %w", err)
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	}

	if len(allPatterns) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No patterns found. Run 'repited patterns init' first.")
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "Workflow Patterns (%d total):\n\n", len(allPatterns))

	// Group by category
	grouped := make(map[string][]patterns.Pattern)
	for _, p := range allPatterns {
		grouped[p.Category] = append(grouped[p.Category], p)
	}

	for cat, pats := range grouped {
		_, _ = fmt.Fprintf(os.Stdout, "  %s %s (%d)\n", categoryIcon(cat), strings.ToUpper(cat), len(pats))

		for _, p := range pats {
			conf := ""
			if p.Confidence > 0 {
				conf = fmt.Sprintf(" (%.0f%%)", p.Confidence*100)
			}

			sourceTag := sourceLabel(p.Source)
			_, _ = fmt.Fprintf(os.Stdout, "    %-45s %s %s%s\n", p.Name, sourceTag, p.Source, conf)
		}

		_, _ = fmt.Fprintln(os.Stdout)
	}

	return nil
}

func runPatternsSuggest(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	asJSON, _ := cmd.Flags().GetBool("json")

	ps := patterns.Default()

	suggestions, err := ps.SuggestFlows(dir)
	if err != nil {
		return fmt.Errorf("suggesting flows: %w", err)
	}

	if asJSON {
		data, err := json.MarshalIndent(suggestions, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling suggestions: %w", err)
		}

		_, _ = fmt.Fprintln(os.Stdout, string(data))

		return nil
	}

	if len(suggestions) == 0 {
		_, _ = fmt.Fprintf(os.Stdout, "No matching patterns for %s\n", dir)
		_, _ = fmt.Fprintln(os.Stdout, "Run 'repited patterns init' and 'repited patterns detect' first.")

		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "Suggested workflows for %s:\n\n", dir)

	for i, p := range suggestions {
		icon := categoryIcon(p.Category)
		_, _ = fmt.Fprintf(os.Stdout, "  %d. %s %s\n", i+1, icon, p.Name)
		_, _ = fmt.Fprintf(os.Stdout, "     %s\n", p.Description)

		stepNames := make([]string, len(p.Steps))
		for j, s := range p.Steps {
			stepNames[j] = s.Tool
		}

		_, _ = fmt.Fprintf(os.Stdout, "     Steps: %s\n", strings.Join(stepNames, " → "))
		_, _ = fmt.Fprintln(os.Stdout)
	}

	return nil
}

func runPatternsCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	tools, _ := cmd.Flags().GetString("tools")
	category, _ := cmd.Flags().GetString("category")
	description, _ := cmd.Flags().GetString("description")

	toolList := strings.Split(tools, ",")
	steps := make([]patterns.Step, len(toolList))

	for i, t := range toolList {
		steps[i] = patterns.Step{
			Tool:   strings.TrimSpace(t),
			Order:  i + 1,
			OnFail: "stop",
		}
	}

	if description == "" {
		description = fmt.Sprintf("User-defined pattern: %s", name)
	}

	p := patterns.Pattern{
		ID:          fmt.Sprintf("user-%s", patterns.SanitizeID(name)),
		Name:        name,
		Description: description,
		Category:    category,
		Steps:       steps,
		Confidence:  1.0,
		Occurrences: 0,
		Source:      "user-defined",
		DetectedAt:  time.Now(),
	}

	ps := patterns.Default()

	if err := ps.SaveUserPattern(p); err != nil {
		return fmt.Errorf("saving user pattern: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Created user pattern %q with %d steps in category %q\n", name, len(steps), category)
	_, _ = fmt.Fprintf(os.Stdout, "Saved to %s\n", ps.UserPatternsFile())

	return nil
}

func runPatternsDelete(_ *cobra.Command, args []string) error {
	name := args[0]

	if patterns.IsBuiltinPattern(name) {
		return fmt.Errorf("cannot delete builtin pattern %q", name)
	}

	ps := patterns.Default()

	if err := ps.DeleteUserPattern(name); err != nil {
		return fmt.Errorf("deleting user pattern: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Deleted user pattern %q\n", name)

	return nil
}

func runPatternsRules(cmd *cobra.Command, _ []string) error {
	ps := patterns.Default()

	rules, err := ps.LoadRules()
	if err != nil {
		return fmt.Errorf("loading rules: %w", err)
	}

	if len(rules) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No rules found. Run 'repited patterns init' first.")
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "Rules (%d total):\n\n", len(rules))

	// Group by category
	grouped := make(map[string][]patterns.Rule)
	for _, r := range rules {
		grouped[r.Category] = append(grouped[r.Category], r)
	}

	for cat, rs := range grouped {
		_, _ = fmt.Fprintf(os.Stdout, "  %s\n", strings.ToUpper(cat))

		for _, r := range rs {
			status := "ON "
			if !r.Enabled {
				status = "OFF"
			}

			sev := severityIcon(r.Severity)
			_, _ = fmt.Fprintf(os.Stdout, "    [%s] %s %-40s %s\n", status, sev, r.Name, r.Description)
			_, _ = fmt.Fprintf(os.Stdout, "         Check: %s\n", r.Check)
		}

		_, _ = fmt.Fprintln(os.Stdout)
	}

	dir := cmdlog.DataDir()
	_, _ = fmt.Fprintf(os.Stdout, "Rules stored at: %s\n", dir)

	return nil
}

func categoryIcon(cat string) string {
	switch cat {
	case "flow":
		return "[F]"
	case "guard":
		return "[G]"
	case "deploy":
		return "[D]"
	case "test":
		return "[T]"
	case "setup":
		return "[S]"
	case "refactor":
		return "[R]"
	default:
		return "[?]"
	}
}

func severityIcon(sev string) string {
	switch sev {
	case "error":
		return "ERR"
	case "warning":
		return "WRN"
	case "info":
		return "INF"
	default:
		return "   "
	}
}

func sourceLabel(source string) string {
	switch source {
	case "builtin":
		return "[builtin]"
	case "user-defined":
		return "[user]"
	default:
		return "[detected]"
	}
}
