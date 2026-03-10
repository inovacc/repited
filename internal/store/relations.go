package store

import (
	"fmt"
)

// ToolRelation represents a directional relationship: tool A appears before tool B in a script.
type ToolRelation struct {
	From  string
	To    string
	Count int // number of scripts where From precedes To
}

// ToolCooccurrence represents two tools appearing in the same script.
type ToolCooccurrence struct {
	ToolA string
	ToolB string
	Count int // number of scripts where both appear
}

// WorkflowStep represents a tool and how often it appears at a given position in scripts.
type WorkflowStep struct {
	Tool     string
	Position string // "first", "middle", "last"
	Count    int
}

// ToolCluster groups tools that frequently appear together.
type ToolCluster struct {
	Category string
	Tools    []ClusterTool
}

// ClusterTool is a tool within a cluster.
type ClusterTool struct {
	Tool  string
	Count int
}

// ToolSequences returns directional pairs: "A then B" within the same script.
// Only counts each unique pair once per script, preserving order.
func (s *Store) ToolSequences(scanID int64, minCount int, limit int) ([]ToolRelation, error) {
	rows, err := s.db.Query(`
		WITH script_cmds AS (
			SELECT c.script_id, c.tool, c.id as cmd_order
			FROM commands c
			JOIN scripts s ON s.id = c.script_id
			JOIN projects p ON p.id = s.project_id
			WHERE p.scan_id = ?
		),
		pairs AS (
			SELECT DISTINCT a.script_id, a.tool AS from_tool, b.tool AS to_tool
			FROM script_cmds a
			JOIN script_cmds b ON a.script_id = b.script_id
				AND a.cmd_order < b.cmd_order
				AND a.tool != b.tool
		)
		SELECT from_tool, to_tool, COUNT(*) as cnt
		FROM pairs
		GROUP BY from_tool, to_tool
		HAVING cnt >= ?
		ORDER BY cnt DESC
		LIMIT ?`,
		scanID, minCount, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying tool sequences: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var rels []ToolRelation

	for rows.Next() {
		var r ToolRelation
		if err := rows.Scan(&r.From, &r.To, &r.Count); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		rels = append(rels, r)
	}

	return rels, rows.Err()
}

// ToolCooccurrences returns undirected pairs: tools that appear in the same script.
func (s *Store) ToolCooccurrences(scanID int64, minCount int, limit int) ([]ToolCooccurrence, error) {
	rows, err := s.db.Query(`
		WITH script_tools AS (
			SELECT DISTINCT c.script_id, c.tool
			FROM commands c
			JOIN scripts s ON s.id = c.script_id
			JOIN projects p ON p.id = s.project_id
			WHERE p.scan_id = ?
		)
		SELECT a.tool, b.tool, COUNT(*) as cnt
		FROM script_tools a
		JOIN script_tools b ON a.script_id = b.script_id
			AND a.tool < b.tool
		GROUP BY a.tool, b.tool
		HAVING cnt >= ?
		ORDER BY cnt DESC
		LIMIT ?`,
		scanID, minCount, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying cooccurrences: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var pairs []ToolCooccurrence

	for rows.Next() {
		var p ToolCooccurrence
		if err := rows.Scan(&p.ToolA, &p.ToolB, &p.Count); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		pairs = append(pairs, p)
	}

	return pairs, rows.Err()
}

// ToolPositions analyzes where tools tend to appear: start, middle, or end of scripts.
func (s *Store) ToolPositions(scanID int64, limit int) ([]WorkflowStep, error) {
	rows, err := s.db.Query(`
		WITH ranked AS (
			SELECT
				c.tool,
				c.script_id,
				ROW_NUMBER() OVER (PARTITION BY c.script_id ORDER BY c.id ASC) as pos,
				COUNT(*) OVER (PARTITION BY c.script_id) as total
			FROM commands c
			JOIN scripts s ON s.id = c.script_id
			JOIN projects p ON p.id = s.project_id
			WHERE p.scan_id = ?
				AND s.command_count >= 2
		),
		positioned AS (
			SELECT tool,
				CASE
					WHEN pos = 1 THEN 'first'
					WHEN pos = total THEN 'last'
					ELSE 'middle'
				END as position
			FROM ranked
		)
		SELECT tool, position, COUNT(*) as cnt
		FROM positioned
		GROUP BY tool, position
		HAVING cnt >= 3
		ORDER BY cnt DESC
		LIMIT ?`,
		scanID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying tool positions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var steps []WorkflowStep

	for rows.Next() {
		var ws WorkflowStep
		if err := rows.Scan(&ws.Tool, &ws.Position, &ws.Count); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		steps = append(steps, ws)
	}

	return steps, rows.Err()
}

// ToolClusters groups tools into categories based on their prefix or known groupings.
func (s *Store) ToolClusters(scanID int64) ([]ToolCluster, error) {
	categories := map[string]string{
		"git":           "Git (Version Control)",
		"go":            "Go (Build & Test)",
		"gh":            "GitHub CLI",
		"docker":        "Docker",
		"kubectl":       "Kubernetes",
		"terraform":     "Terraform",
		"tf":            "Terraform",
		"npm":           "Node.js",
		"npx":           "Node.js",
		"cargo":         "Rust",
		"pip":           "Python",
		"python":        "Python",
		"task":          "Task Runner",
		"omni":          "Omni (Cross-Platform)",
		"golangci-lint": "Go (Build & Test)",
		"curl":          "HTTP & Networking",
		"wget":          "HTTP & Networking",
	}

	rows, err := s.db.Query(`
		SELECT tool, count FROM tool_counts
		WHERE scan_id = ? AND count >= 3
		ORDER BY count DESC`,
		scanID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying tool counts: %w", err)
	}

	defer func() { _ = rows.Close() }()

	clusterMap := make(map[string][]ClusterTool)

	for rows.Next() {
		var (
			tool  string
			count int
		)

		if err := rows.Scan(&tool, &count); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		cat := categorize(tool, categories)
		clusterMap[cat] = append(clusterMap[cat], ClusterTool{Tool: tool, Count: count})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort clusters by total count descending
	var clusters []ToolCluster
	for cat, tools := range clusterMap {
		clusters = append(clusters, ToolCluster{Category: cat, Tools: tools})
	}

	// Sort by sum of counts in each cluster
	for i := 0; i < len(clusters); i++ {
		for j := i + 1; j < len(clusters); j++ {
			if clusterSum(clusters[j]) > clusterSum(clusters[i]) {
				clusters[i], clusters[j] = clusters[j], clusters[i]
			}
		}
	}

	return clusters, nil
}

func categorize(tool string, categories map[string]string) string {
	// Check exact match first
	if cat, ok := categories[tool]; ok {
		return cat
	}

	// Check prefix match (e.g., "git add" -> "git")
	for prefix, cat := range categories {
		if len(tool) > len(prefix) && tool[:len(prefix)+1] == prefix+" " {
			return cat
		}
	}

	// Shell utilities
	shellUtils := map[string]bool{
		"grep": true, "sed": true, "awk": true, "head": true, "tail": true,
		"cat": true, "ls": true, "find": true, "wc": true, "sort": true,
		"uniq": true, "cut": true, "xargs": true, "tee": true, "diff": true,
	}
	if shellUtils[tool] {
		return "Shell Utilities"
	}

	// File operations
	fileOps := map[string]bool{
		"cp": true, "mv": true, "rm": true, "mkdir": true, "touch": true,
		"chmod": true, "chown": true, "ln": true, "basename": true, "dirname": true,
	}
	if fileOps[tool] {
		return "File Operations"
	}

	return "Other"
}

func clusterSum(c ToolCluster) int {
	total := 0
	for _, t := range c.Tools {
		total += t.Count
	}

	return total
}
