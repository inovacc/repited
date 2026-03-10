# Feature Requests

## Completed Features

### Directory Scanner
- **Status:** Completed
- **Description:** Walk directories to find .git + .scripts pairs, parse shell scripts, extract commands
- **Includes:** Multi-word command tracking, noise filtering, pipeline splitting

### SQLite Persistence
- **Status:** Completed
- **Description:** Store scan results in SQLite with 5 tables (scans, projects, scripts, commands, tool_counts)

### Command Relations
- **Status:** Completed
- **Description:** Analyze co-occurrence, sequences, positions, and clusters from stored scan data

### Flow Pipeline
- **Status:** Completed
- **Description:** Auto-detected build→test→lint→stage→commit workflow for Go/Node.js/Rust
- **Includes:** Skip/only flags, dry-run, commit message, push option

### MCP Server
- **Status:** Completed
- **Description:** stdio MCP server exposing 7 tools for Claude Code integration
- **Includes:** Auto-install via `mcp install --global --client claude`

### Pattern Detection
- **Status:** Completed
- **Description:** AI-informed heuristics detect workflow patterns from scan data
- **Includes:** 10 builtin patterns, 12 builtin rules, flow/guard/teardown/cluster detection

### Scout Integration
- **Status:** Completed
- **Description:** Browser automation wrapper delegating to scout binary or omni curl

### Next-Steps Suggestions
- **Status:** Completed
- **Description:** Context-aware suggestions after push/commit/merge/release/deploy/test/refactor

### Pre-commit Hook
- **Status:** Completed
- **Description:** Git hook running lint:fix, vet, build before every commit

### Auto-install Dependencies
- **Status:** Completed
- **Description:** Auto-install omni and scout via go install if not found

## Proposed Features

### JSON/CSV Output Formats
- **Priority:** P2
- **Status:** Proposed
- **Description:** Output scan/stats results as JSON or CSV for scripting

### Interactive TUI
- **Priority:** P3
- **Status:** Proposed
- **Description:** Terminal UI for browsing scan results and patterns

### Pattern Sharing
- **Priority:** P3
- **Status:** Proposed
- **Description:** Export/import patterns between teams and projects
