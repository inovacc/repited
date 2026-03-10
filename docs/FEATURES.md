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

### JSON Output Format
- **Status:** Completed
- **Description:** `--json` flag on scan and stats commands for machine-readable output

### Tool Filter
- **Status:** Completed
- **Description:** `--filter` flag on stats and relations commands for narrowing results by tool name

### Project-Level MCP Install
- **Status:** Completed
- **Description:** `mcp install --project` writes .mcp.json for per-project MCP config

### Exclude Patterns
- **Status:** Completed
- **Description:** `--exclude` flag and DefaultExcludes (node_modules, vendor, etc.) for scanner

### v1.0.0 Release
- **Status:** Completed
- **Published:** 2026-03-10
- **Description:** First stable release via goreleaser with 6 platform binaries
- **Includes:** All features above, CI/CD pipeline, 80%+ test coverage

### Pattern Editing CLI
- **Status:** Completed
- **Description:** Enable, disable, and edit pattern rules from the command line
- **Includes:** `patterns edit`, `patterns enable`, `patterns disable` subcommands

### Custom User-Defined Patterns
- **Status:** Completed
- **Description:** Create and delete custom workflow patterns with source tags
- **Includes:** `patterns create`, `patterns delete` subcommands, source tags in `patterns list`

### PowerShell Script Parsing
- **Status:** Completed
- **Description:** Parse `.ps1` scripts alongside `.sh`/`.bash`, with cmdlet filtering
- **Includes:** PowerShell cmdlet recognition, noise filtering for PS-specific syntax

### Python Script Parsing
- **Status:** Completed
- **Description:** Parse `.py` scripts for subprocess calls, os.system invocations, and `!` shell commands
- **Includes:** Python-specific noise filtering, subprocess.run/call/Popen detection

### Watch Mode
- **Status:** Completed
- **Description:** File system watcher that rescans on changes to `.scripts/` directories
- **Includes:** `scan --watch` flag, fsnotify integration, debouncing to avoid duplicate scans

### Pattern Sharing
- **Status:** Completed
- **Description:** Export and import patterns between teams and projects
- **Includes:** `patterns export`, `patterns import` with skip/merge/overwrite conflict modes

## Proposed Features

### Interactive TUI
- **Priority:** P3
- **Status:** Proposed
- **Description:** Terminal UI for browsing scan results and patterns
