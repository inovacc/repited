# Milestones

## v0.1.0 - Foundation
- **Status:** Complete
- **Test Coverage:** scanner 92.4%, store 85.4%, cmdlog 92.5%
- **Goals:**
  - [x] Project scaffolding
  - [x] Directory scanner (find .git + .scripts)
  - [x] Shell command extractor (multi-word commands, pipelines, noise filtering)
  - [x] CLI scan command with table output
  - [x] SQLite persistence (5 tables: scans, projects, scripts, commands, tool_counts)
  - [x] Unit tests for core packages

## v0.2.0 - Analysis & Workflows
- **Status:** Complete
- **Goals:**
  - [x] Stats command (query stored scan data)
  - [x] Relations command (sequences, co-occurrence, positions, clusters)
  - [x] Flow command (build→test→lint→stage→commit pipeline)
  - [x] Auto-detect Go/Node.js/Rust projects
  - [x] KSUID command logging (AppData\Local\Repited\commands\)

## v0.3.0 - MCP Server & Patterns
- **Status:** Complete
- **Test Coverage:** mcp 49.9%, patterns 79.3%, deps 92.5%
- **Goals:**
  - [x] MCP server with stdio transport (7 tools: flow, scan, stats, relations, patterns, scout, next-steps)
  - [x] `mcp install --global --client claude` writes config to ~/.claude.json
  - [x] Pattern detection from scan data (flows, guards, teardowns, clusters)
  - [x] Builtin patterns (10 workflows) and rules (12 rules)
  - [x] Scout tool for browser automation
  - [x] Next-steps tool for post-action suggestions
  - [x] Auto-install dependencies (omni, scout)
  - [x] Git pre-commit hook (lint:fix + vet + build)

## v1.0.0 - First Stable Release
- **Published:** 2026-03-10
- **Status:** Complete
- **Goals:**
  - [x] Unit tests for 7 packages (~81% overall)
  - [x] Integration tests for MCP server (49.9% — 13 tests via in-memory transports)
  - [x] Documentation complete
  - [x] CI/CD pipeline (GitHub Actions)
  - [x] Published to GitHub (github.com/inovacc/repited)
  - [x] Performance tuning (--exclude flag, DefaultExcludes, benchmarks)
  - [x] JSON output for scan/stats commands
  - [x] --filter flag for stats/relations
  - [x] MCP project-level install (--project flag)
  - [x] goreleaser release (v1.0.0 — 6 platform binaries published)

## v1.1.0 - Feature Expansion
- **Published:** 2026-03-10
- **Status:** Complete
- **Goals:**
  - [x] Pattern editing CLI (enable/disable/edit commands)
  - [x] Custom user-defined patterns (create/delete + source tags in list)
  - [x] PowerShell script parsing (.ps1 with cmdlet filtering)
  - [x] Python script parsing (.py — subprocess, os.system, ! commands)
  - [x] Watch mode (scan --watch with fsnotify and debouncing)
  - [x] Pattern sharing (export/import with skip/merge/overwrite modes)

## v1.2.0 - TUI & Extensibility
- **Status:** Planned
- **Goals:**
  - [ ] Interactive TUI mode (browse scan results and patterns)
  - [ ] Remote pattern registry
  - [ ] Plugin system for custom analyzers
