package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/inovacc/repited/internal/scanner"
	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database for persisting scan results.
type Store struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS scans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			root_dir TEXT NOT NULL,
			scanned_at TEXT NOT NULL,
			project_count INTEGER NOT NULL,
			tool_count INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scan_id INTEGER NOT NULL,
			path TEXT NOT NULL,
			script_count INTEGER NOT NULL,
			FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS scripts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			command_count INTEGER NOT NULL,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS commands (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			script_id INTEGER NOT NULL,
			tool TEXT NOT NULL,
			FOREIGN KEY (script_id) REFERENCES scripts(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS tool_counts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scan_id INTEGER NOT NULL,
			tool TEXT NOT NULL,
			count INTEGER NOT NULL,
			FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_scan_id ON projects(scan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_scripts_project_id ON scripts(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_commands_script_id ON commands(script_id)`,
		`CREATE INDEX IF NOT EXISTS idx_commands_tool ON commands(tool)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_counts_scan_id ON tool_counts(scan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_counts_tool ON tool_counts(tool)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}

	return nil
}

// SaveScan persists a full scan result into the database within a single transaction.
func (s *Store) SaveScan(rootDir string, result *scanner.ScanResult) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Insert scan record
	scanRes, err := tx.Exec(
		`INSERT INTO scans (root_dir, scanned_at, project_count, tool_count) VALUES (?, ?, ?, ?)`,
		rootDir,
		time.Now().UTC().Format(time.RFC3339),
		len(result.Projects),
		len(result.ToolCounts),
	)
	if err != nil {
		return 0, fmt.Errorf("inserting scan: %w", err)
	}

	scanID, err := scanRes.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting scan id: %w", err)
	}

	// Insert tool counts
	toolStmt, err := tx.Prepare(`INSERT INTO tool_counts (scan_id, tool, count) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing tool_counts insert: %w", err)
	}

	defer func() { _ = toolStmt.Close() }()

	for _, tc := range result.ToolCounts {
		if _, err := toolStmt.Exec(scanID, tc.Name, tc.Count); err != nil {
			return 0, fmt.Errorf("inserting tool count %q: %w", tc.Name, err)
		}
	}

	// Insert projects, scripts, and commands
	projStmt, err := tx.Prepare(`INSERT INTO projects (scan_id, path, script_count) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing projects insert: %w", err)
	}

	defer func() { _ = projStmt.Close() }()

	scriptStmt, err := tx.Prepare(`INSERT INTO scripts (project_id, name, path, command_count) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing scripts insert: %w", err)
	}

	defer func() { _ = scriptStmt.Close() }()

	cmdStmt, err := tx.Prepare(`INSERT INTO commands (script_id, tool) VALUES (?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing commands insert: %w", err)
	}

	defer func() { _ = cmdStmt.Close() }()

	for _, proj := range result.Projects {
		projRes, err := projStmt.Exec(scanID, proj.Path, len(proj.Scripts))
		if err != nil {
			return 0, fmt.Errorf("inserting project %q: %w", proj.Path, err)
		}

		projID, err := projRes.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("getting project id: %w", err)
		}

		for _, script := range proj.Scripts {
			scriptRes, err := scriptStmt.Exec(projID, script.Name, script.Path, len(script.Commands))
			if err != nil {
				return 0, fmt.Errorf("inserting script %q: %w", script.Name, err)
			}

			scriptID, err := scriptRes.LastInsertId()
			if err != nil {
				return 0, fmt.Errorf("getting script id: %w", err)
			}

			for _, cmd := range script.Commands {
				if _, err := cmdStmt.Exec(scriptID, cmd); err != nil {
					return 0, fmt.Errorf("inserting command %q: %w", cmd, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return scanID, nil
}
