# Milestones

## v0.1.0 - Foundation
- **Status:** Complete
- **Test Coverage:** 0.0% (no tests yet)
- **Goals:**
  - [x] Project scaffolding
  - [x] Directory scanner (find .git + .scripts)
  - [x] Shell command extractor (multi-word commands, pipelines, noise filtering)
  - [x] CLI scan command with table output
  - [x] SQLite persistence (5 tables: scans, projects, scripts, commands, tool_counts)
  - [ ] Unit tests (80%+ coverage)

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
- **Status:** Not Started
- **Goals:**
  - [ ] Full test coverage (80%+)
  - [ ] Integration tests for MCP server
  - [ ] Documentation complete
  - [ ] CI/CD pipeline (GitHub Actions)
  - [ ] Published to GitHub with goreleaser
