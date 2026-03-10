package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/inovacc/repited/internal/store"
	"github.com/inovacc/repited/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open interactive TUI dashboard",
	Long: `Launch an interactive terminal UI that displays scan statistics,
tool frequency, and detected patterns from the SQLite database.

Navigation:
  tab / shift+tab   cycle between views
  d / s / t / p      jump to Dashboard / Scans / Tools / Patterns
  ?                  toggle help
  q                  quit`,
	RunE: runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)

	tuiCmd.Flags().String("db", defaultDBPath(), "SQLite database path")
}

func runTUI(cmd *cobra.Command, _ []string) error {
	dbPath, _ := cmd.Flags().GetString("db")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database not found at %s — run 'repited scan' first", dbPath)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	defer func() { _ = db.Close() }()

	model, err := tui.NewModel(db)
	if err != nil {
		return fmt.Errorf("initializing TUI: %w", err)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
