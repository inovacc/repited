package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/inovacc/repited/internal/patterns"
	"github.com/inovacc/repited/internal/store"
)

// ── styles ──

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	valueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229"))

	barStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	categoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
)

// ── Dashboard view ──

func renderDashboard(d data, width int) string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Summary"))
	b.WriteString("\n\n")

	// Stat cards
	stats := []struct {
		label string
		value int
	}{
		{"Total Scans", d.stats.TotalScans},
		{"Projects", d.stats.TotalProjects},
		{"Scripts", d.stats.TotalScripts},
		{"Commands", d.stats.TotalCommands},
		{"Unique Tools", d.stats.UniqueTools},
	}

	statParts := make([]string, 0, len(stats))

	for _, s := range stats {
		card := fmt.Sprintf("%s %s",
			labelStyle.Render(s.label+":"),
			valueStyle.Render(fmt.Sprintf("%d", s.value)),
		)
		statParts = append(statParts, card)
	}

	b.WriteString(strings.Join(statParts, "  |  "))
	b.WriteString("\n\n")

	// Top 10 tools bar chart
	limit := min(10, len(d.tools))

	if limit > 0 {
		b.WriteString(headerStyle.Render("Top 10 Tools"))
		b.WriteString("\n\n")

		maxCount := d.tools[0].Count
		maxBarWidth := max(width-30, 10)

		for i := range limit {
			tc := d.tools[i]
			barLen := max((tc.Count*maxBarWidth)/maxCount, 1)

			name := fmt.Sprintf("%-20s", truncate(tc.Tool, 20))
			bar := strings.Repeat("█", barLen)
			count := fmt.Sprintf(" %d", tc.Count)

			_, _ = fmt.Fprintf(&b, "  %s %s%s\n",
				labelStyle.Render(name),
				barStyle.Render(bar),
				dimStyle.Render(count),
			)
		}
	}

	// Recent scans
	scanLimit := min(5, len(d.scans))

	if scanLimit > 0 {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Recent Scans"))
		b.WriteString("\n\n")

		for i := range scanLimit {
			sc := d.scans[i]

			_, _ = fmt.Fprintf(&b, "  %s  %s  %s  %s\n",
				dimStyle.Render(fmt.Sprintf("#%-4d", sc.ID)),
				labelStyle.Render(truncate(sc.RootDir, 30)),
				dimStyle.Render(sc.ScannedAt),
				valueStyle.Render(fmt.Sprintf("%d projects, %d tools", sc.ProjectCount, sc.ToolCount)),
			)
		}
	}

	return b.String()
}

// ── Scans view ──

func renderScans(scans []store.ScanSummary, width int) string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("All Scans"))
	b.WriteString("\n\n")

	if len(scans) == 0 {
		b.WriteString(dimStyle.Render("  No scans found. Run 'repited scan' first."))

		return b.String()
	}

	// Header row
	row := fmt.Sprintf("  %-6s %-30s %-22s %-10s %-10s",
		"ID", "ROOT DIR", "SCANNED AT", "PROJECTS", "TOOLS")
	b.WriteString(labelStyle.Render(row))
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("─", min(width-4, 80)))
	b.WriteString("\n")

	for _, sc := range scans {
		rootDir := truncate(sc.RootDir, 30)

		_, _ = fmt.Fprintf(&b, "  %-6d %-30s %-22s %-10d %-10d\n",
			sc.ID, rootDir, sc.ScannedAt, sc.ProjectCount, sc.ToolCount)
	}

	return b.String()
}

// ── Tools view ──

func renderTools(tools []store.StoredToolCount, width, height int) string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Tool Frequency"))
	b.WriteString("\n\n")

	if len(tools) == 0 {
		b.WriteString(dimStyle.Render("  No tool data available."))

		return b.String()
	}

	// Header
	row := fmt.Sprintf("  %-5s %-25s %-8s %s",
		"RANK", "TOOL", "COUNT", "BAR")
	b.WriteString(labelStyle.Render(row))
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("─", min(width-4, 80)))
	b.WriteString("\n")

	maxCount := tools[0].Count
	maxBarWidth := max(width-45, 5)

	// Limit rows to available height.
	limit := min(len(tools), height-4)

	if limit < 1 {
		limit = len(tools)
	}

	for i := range limit {
		tc := tools[i]
		barLen := max((tc.Count*maxBarWidth)/maxCount, 1)

		bar := barStyle.Render(strings.Repeat("█", barLen))
		_, _ = fmt.Fprintf(&b, "  %-5d %-25s %-8d %s\n",
			i+1, truncate(tc.Tool, 25), tc.Count, bar)
	}

	if len(tools) > limit {
		_, _ = fmt.Fprintf(&b, "\n  %s",
			dimStyle.Render(fmt.Sprintf("... and %d more tools", len(tools)-limit)))
	}

	return b.String()
}

// ── Patterns view ──

func renderPatterns(pats []patterns.Pattern, width, height int) string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Detected Patterns"))
	b.WriteString("\n\n")

	if len(pats) == 0 {
		b.WriteString(dimStyle.Render("  No patterns found. Run 'repited patterns init' and 'repited patterns detect' first."))

		return b.String()
	}

	limit := min(len(pats), height-4)

	if limit < 1 {
		limit = len(pats)
	}

	for i := range limit {
		p := pats[i]

		// Pattern header
		cat := categoryStyle.Render(fmt.Sprintf("[%s]", p.Category))
		conf := dimStyle.Render(fmt.Sprintf("%.0f%%", p.Confidence*100))
		_, _ = fmt.Fprintf(&b, "  %s %s %s\n", cat, valueStyle.Render(p.Name), conf)

		// Tools in the pattern
		var toolNames []string

		for _, s := range p.Steps {
			toolNames = append(toolNames, s.Tool)
		}

		toolLine := truncate(strings.Join(toolNames, " -> "), width-8)
		_, _ = fmt.Fprintf(&b, "    %s\n", dimStyle.Render(toolLine))

		if i < limit-1 {
			b.WriteString("\n")
		}
	}

	if len(pats) > limit {
		_, _ = fmt.Fprintf(&b, "\n  %s",
			dimStyle.Render(fmt.Sprintf("... and %d more patterns", len(pats)-limit)))
	}

	return b.String()
}

// ── helpers ──

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return s[:maxLen]
	}

	return s[:maxLen-3] + "..."
}
