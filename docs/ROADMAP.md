# Roadmap

## Current Status
**Overall Progress:** v1.1.0 complete

## Phases

### Phase 1: Foundation [COMPLETE]
- [x] Project scaffold (structure, tooling, CI config)
- [x] Core scanner: walk directories for .git + .scripts
- [x] Shell script parser: extract commands from .sh files
- [x] CLI `scan` command with flags (depth, top, projects)
- [x] SQLite persistence (scans, projects, scripts, commands, tool_counts)
- [x] Stats command (query stored data, top tools, project breakdowns)
- [x] Unit tests for scanner and parser (92.4% coverage)

### Phase 2: Analysis & Workflows [COMPLETE]
- [x] Command co-occurrence analysis (SQL self-joins)
- [x] Sequential patterns (A→B within same script)
- [x] Tool position analysis (first/middle/last)
- [x] Tool clusters by category (Git, Go, Docker, K8s, etc.)
- [x] Flow command: build→test→lint→stage→commit pipeline
- [x] Auto-detect project type (Go, Node.js, Rust)
- [x] KSUID-named command logging to AppData\Local\Repited\commands\

### Phase 3: MCP Server & Patterns [COMPLETE]
- [x] MCP server (stdio transport) with 7 tools
- [x] `mcp install --global --client claude` (writes ~/.claude.json)
- [x] Pattern detection engine (flows, guards, teardowns, clusters)
- [x] Builtin patterns (Go, Node, Rust, Docker, K8s, Terraform, PR, release)
- [x] Builtin rules (pre-commit, quality, security, convention)
- [x] Scout tool (browser automation wrapper)
- [x] Next-steps tool (post-action suggestions)
- [x] Auto-install dependencies (omni, scout)
- [x] Git pre-commit hook (lint:fix + vet + build)

### Phase 4: Polish & Release [COMPLETE]
- [x] Unit tests for scanner (92.8%), store (85.4%), flow (88.2%), patterns (79.3%), cmdlog (92.5%), deps (92.5%), mcp (49.9%)
- [x] Integration tests for MCP server (49.9% — 13 integration tests via in-memory transports)
- [x] Performance tuning (--exclude flag, DefaultExcludes, BenchmarkScan ~28ms/100 projects)
- [x] CI/CD pipeline (GitHub Actions — workflows exist, repo pushed)
- [x] Published to GitHub (github.com/inovacc/repited)
- [x] v1.0.0 release (goreleaser — 6 platform binaries)

### Phase 5: v1.1.0 Features [COMPLETE]
- [x] Pattern editing CLI (enable/disable/edit commands)
- [x] Custom user-defined patterns (create/delete + source tags)
- [x] PowerShell script parsing (.ps1 with cmdlet filtering)
- [x] Python script parsing (.py — subprocess, os.system, ! commands)
- [x] Watch mode (scan --watch with fsnotify and debouncing)
- [x] Pattern sharing (export/import with skip/merge/overwrite modes)

### Phase 6: v1.2.0 Enhancements [PLANNED]
- [ ] Interactive TUI mode (browse scan results and patterns)
- [ ] Remote pattern registry
- [ ] Plugin system for custom analyzers

## Test Coverage
**Current:** ~85%  |  **Target:** 80%

| Package | Coverage | Status |
|---------|----------|--------|
| internal/scanner | 92.8% | Excellent |
| internal/cmdlog | 92.5% | Excellent |
| internal/deps | 92.5% | Excellent |
| internal/flow | 88.2% | Good |
| internal/patterns | 79.3% | Good |
| internal/store | 85.4% | Good |
| internal/mcp | 74.4% | Good |
| cmd | 0.0% | No tests (CLI wrappers) |
