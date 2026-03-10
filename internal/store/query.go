package store

import (
	"fmt"
)

// ScanSummary holds basic info about a stored scan.
type ScanSummary struct {
	ID           int64
	RootDir      string
	ScannedAt    string
	ProjectCount int
	ToolCount    int
}

// ListScans returns all stored scan summaries ordered by most recent first.
func (s *Store) ListScans() ([]ScanSummary, error) {
	rows, err := s.db.Query(`SELECT id, root_dir, scanned_at, project_count, tool_count FROM scans ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying scans: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var scans []ScanSummary

	for rows.Next() {
		var sc ScanSummary
		if err := rows.Scan(&sc.ID, &sc.RootDir, &sc.ScannedAt, &sc.ProjectCount, &sc.ToolCount); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		scans = append(scans, sc)
	}

	return scans, rows.Err()
}

// StoredToolCount holds a tool name and its occurrence count from a specific scan.
type StoredToolCount struct {
	Tool  string
	Count int
}

// TopToolsByScan returns top tools for a given scan ID.
func (s *Store) TopToolsByScan(scanID int64, limit int) ([]StoredToolCount, error) {
	rows, err := s.db.Query(
		`SELECT tool, count FROM tool_counts WHERE scan_id = ? ORDER BY count DESC LIMIT ?`,
		scanID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying tool counts: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var tools []StoredToolCount

	for rows.Next() {
		var tc StoredToolCount
		if err := rows.Scan(&tc.Tool, &tc.Count); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		tools = append(tools, tc)
	}

	return tools, rows.Err()
}

// StoredProject holds a project record retrieved from the database.
type StoredProject struct {
	ID          int64
	Path        string
	ScriptCount int
}

func (s *Store) ProjectsByScan(scanID int64) ([]StoredProject, error) {
	rows, err := s.db.Query(
		`SELECT id, path, script_count FROM projects WHERE scan_id = ? ORDER BY script_count DESC`,
		scanID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying projects: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var projects []StoredProject

	for rows.Next() {
		var p StoredProject
		if err := rows.Scan(&p.ID, &p.Path, &p.ScriptCount); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		projects = append(projects, p)
	}

	return projects, rows.Err()
}

// ProjectCommandCount holds total command invocations for a single project.
type ProjectCommandCount struct {
	ProjectPath string
	TotalCmds   int
}

func (s *Store) CommandCountByProject(scanID int64) ([]ProjectCommandCount, error) {
	rows, err := s.db.Query(`
		SELECT p.path, COALESCE(SUM(s.command_count), 0) as total_cmds
		FROM projects p
		LEFT JOIN scripts s ON s.project_id = p.id
		WHERE p.scan_id = ?
		GROUP BY p.path
		ORDER BY total_cmds DESC`,
		scanID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying command counts: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var results []ProjectCommandCount

	for rows.Next() {
		var r ProjectCommandCount
		if err := rows.Scan(&r.ProjectPath, &r.TotalCmds); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		results = append(results, r)
	}

	return results, rows.Err()
}

// Stats holds aggregate statistics from the database.
type Stats struct {
	TotalScans    int
	TotalProjects int
	TotalScripts  int
	TotalCommands int
	UniqueTools   int
}

// GetStats returns aggregate statistics across all scans.
func (s *Store) GetStats() (*Stats, error) {
	var stats Stats

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM scans`).Scan(&stats.TotalScans); err != nil {
		return nil, fmt.Errorf("counting scans: %w", err)
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&stats.TotalProjects); err != nil {
		return nil, fmt.Errorf("counting projects: %w", err)
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM scripts`).Scan(&stats.TotalScripts); err != nil {
		return nil, fmt.Errorf("counting scripts: %w", err)
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM commands`).Scan(&stats.TotalCommands); err != nil {
		return nil, fmt.Errorf("counting commands: %w", err)
	}

	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT tool) FROM commands`).Scan(&stats.UniqueTools); err != nil {
		return nil, fmt.Errorf("counting unique tools: %w", err)
	}

	return &stats, nil
}
