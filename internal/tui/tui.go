package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/inovacc/repited/internal/patterns"
	"github.com/inovacc/repited/internal/store"
)

// view represents the active TUI tab.
type view int

const (
	viewDashboard view = iota
	viewScans
	viewTools
	viewPatterns
)

// viewNames maps views to their display labels.
var viewNames = map[view]string{
	viewDashboard: "Dashboard",
	viewScans:     "Scans",
	viewTools:     "Tools",
	viewPatterns:  "Patterns",
}

// viewOrder defines tab ordering for navigation.
var viewOrder = []view{viewDashboard, viewScans, viewTools, viewPatterns}

// data holds all pre-loaded data from the store.
type data struct {
	stats    *store.Stats
	scans    []store.ScanSummary
	tools    []store.StoredToolCount
	patterns []patterns.Pattern
}

// keyMap defines the key bindings.
type keyMap struct {
	Tab      key.Binding
	ShiftTab key.Binding
	Dashboard key.Binding
	Scans    key.Binding
	Tools    key.Binding
	Patterns key.Binding
	Quit     key.Binding
	Help     key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next view")),
		ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev view")),
		Dashboard: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dashboard")),
		Scans:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scans")),
		Tools:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tools")),
		Patterns: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "patterns")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}

// ShortHelp returns key bindings shown in the mini help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Quit, k.Help}
}

// FullHelp returns key bindings shown in the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Dashboard, k.Scans, k.Tools, k.Patterns},
		{k.Tab, k.ShiftTab, k.Quit},
	}
}

// Model is the top-level bubbletea model for the TUI.
type Model struct {
	data     data
	active   view
	width    int
	height   int
	keys     keyMap
	help     help.Model
	showHelp bool
}

// NewModel creates a Model pre-loaded with data from the store.
func NewModel(db *store.Store) (Model, error) {
	var d data

	stats, err := db.GetStats()
	if err != nil {
		return Model{}, fmt.Errorf("loading stats: %w", err)
	}

	d.stats = stats

	scans, err := db.ListScans()
	if err != nil {
		return Model{}, fmt.Errorf("loading scans: %w", err)
	}

	d.scans = scans

	// Load tools from the latest scan.
	if len(scans) > 0 {
		tools, err := db.TopToolsByScan(scans[0].ID, 50)
		if err != nil {
			return Model{}, fmt.Errorf("loading tools: %w", err)
		}

		d.tools = tools
	}

	// Load patterns (best effort).
	ps := patterns.Default()

	loaded, err := ps.LoadPatterns()
	if err == nil {
		d.patterns = loaded
	}

	h := help.New()
	h.ShowAll = false

	return Model{
		data:   d,
		active: viewDashboard,
		keys:   newKeyMap(),
		help:   h,
	}, nil
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update satisfies tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			m.help.ShowAll = m.showHelp
		case key.Matches(msg, m.keys.Tab):
			m.active = nextView(m.active)
		case key.Matches(msg, m.keys.ShiftTab):
			m.active = prevView(m.active)
		case key.Matches(msg, m.keys.Dashboard):
			m.active = viewDashboard
		case key.Matches(msg, m.keys.Scans):
			m.active = viewScans
		case key.Matches(msg, m.keys.Tools):
			m.active = viewTools
		case key.Matches(msg, m.keys.Patterns):
			m.active = viewPatterns
		}
	}

	return m, nil
}

// View satisfies tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Tab bar
	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	// Content area
	contentHeight := m.height - 5 // tabs + help + padding
	content := m.renderContent(contentHeight)
	b.WriteString(content)

	// Help
	b.WriteString("\n")
	b.WriteString(m.help.View(m.keys))

	return b.String()
}

// renderTabs draws the tab bar at the top.
func (m Model) renderTabs() string {
	activeTab := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 2)

	inactiveTab := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Background(lipgloss.Color("236")).
		Padding(0, 2)

	var tabs []string

	for _, v := range viewOrder {
		name := viewNames[v]

		if v == m.active {
			tabs = append(tabs, activeTab.Render(name))
		} else {
			tabs = append(tabs, inactiveTab.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	return row
}

// renderContent dispatches to the active view renderer.
func (m Model) renderContent(height int) string {
	style := lipgloss.NewStyle().
		Width(m.width - 2).
		Height(height).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63"))

	var content string

	switch m.active {
	case viewDashboard:
		content = renderDashboard(m.data, m.width-6)
	case viewScans:
		content = renderScans(m.data.scans, m.width-6)
	case viewTools:
		content = renderTools(m.data.tools, m.width-6, height-4)
	case viewPatterns:
		content = renderPatterns(m.data.patterns, m.width-6, height-4)
	}

	return style.Render(content)
}

func nextView(v view) view {
	for i, vv := range viewOrder {
		if vv == v {
			return viewOrder[(i+1)%len(viewOrder)]
		}
	}

	return viewDashboard
}

func prevView(v view) view {
	for i, vv := range viewOrder {
		if vv == v {
			return viewOrder[(i-1+len(viewOrder))%len(viewOrder)]
		}
	}

	return viewDashboard
}
