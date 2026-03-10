package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/inovacc/repited/internal/patterns"
	"github.com/spf13/cobra"
)

var patternsEnableCmd = &cobra.Command{
	Use:   "enable <rule-name>",
	Short: "Enable a specific builtin rule",
	Args:  cobra.ExactArgs(1),
	RunE:  runPatternsEnable,
}

var patternsDisableCmd = &cobra.Command{
	Use:   "disable <rule-name>",
	Short: "Disable a specific builtin rule",
	Args:  cobra.ExactArgs(1),
	RunE:  runPatternsDisable,
}

var patternsEditCmd = &cobra.Command{
	Use:   "edit <pattern-name>",
	Short: "Modify a pattern's category or tools",
	Long: `Edit an existing pattern's metadata. You can change:
  --category  The pattern category (flow, guard, deploy, test, setup, refactor)
  --tools     Comma-separated list of tool steps (replaces existing steps)

At least one of --category or --tools must be provided.`,
	Args: cobra.ExactArgs(1),
	RunE: runPatternsEdit,
}

func init() {
	patternsCmd.AddCommand(patternsEnableCmd)
	patternsCmd.AddCommand(patternsDisableCmd)
	patternsCmd.AddCommand(patternsEditCmd)

	patternsEditCmd.Flags().StringP("category", "c", "", "new category (flow, guard, deploy, test, setup, refactor)")
	patternsEditCmd.Flags().StringP("tools", "t", "", "comma-separated list of tool steps")
}

func runPatternsEnable(_ *cobra.Command, args []string) error {
	ruleName := args[0]
	ps := patterns.Default()

	err := ps.SetRuleEnabled(ruleName, true)
	if err != nil {
		return fmt.Errorf("enabling rule: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Rule %q enabled.\n", ruleName)

	return nil
}

func runPatternsDisable(_ *cobra.Command, args []string) error {
	ruleName := args[0]
	ps := patterns.Default()

	err := ps.SetRuleEnabled(ruleName, false)
	if err != nil {
		return fmt.Errorf("disabling rule: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Rule %q disabled.\n", ruleName)

	return nil
}

func runPatternsEdit(cmd *cobra.Command, args []string) error {
	patternName := args[0]
	category, _ := cmd.Flags().GetString("category")
	toolsStr, _ := cmd.Flags().GetString("tools")

	if category == "" && toolsStr == "" {
		return fmt.Errorf("at least one of --category or --tools must be provided")
	}

	if category != "" {
		validCategories := map[string]bool{
			"flow": true, "guard": true, "deploy": true,
			"test": true, "setup": true, "refactor": true,
		}

		if !validCategories[category] {
			return fmt.Errorf("invalid category %q: must be one of flow, guard, deploy, test, setup, refactor", category)
		}
	}

	var tools []string

	if toolsStr != "" {
		for t := range strings.SplitSeq(toolsStr, ",") {
			trimmed := strings.TrimSpace(t)

			if trimmed != "" {
				tools = append(tools, trimmed)
			}
		}

		if len(tools) == 0 {
			return fmt.Errorf("--tools must contain at least one non-empty tool name")
		}
	}

	ps := patterns.Default()

	err := ps.EditPattern(patternName, category, tools)
	if err != nil {
		return fmt.Errorf("editing pattern: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Pattern %q updated", patternName)

	if category != "" {
		_, _ = fmt.Fprintf(os.Stdout, " (category: %s)", category)
	}

	if len(tools) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, " (tools: %s)", strings.Join(tools, ", "))
	}

	_, _ = fmt.Fprintln(os.Stdout, ".")

	return nil
}
