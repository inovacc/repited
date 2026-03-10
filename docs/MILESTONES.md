# Milestones

## v0.1.0 - Foundation
- **Status:** Complete
- **Test Coverage:** scanner 92.4%, store 76.1%, cmdlog 92.5%
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
- **Target:** TBD
- **Status:** In Progress
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
  - [ ] goreleaser release (tag v1.0.0)
