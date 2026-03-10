# Roadmap

## Current Status
**Overall Progress:** 75% — MCP server, patterns, and flow pipeline implemented

## Phases

### Phase 1: Foundation [COMPLETE]
- [x] Project scaffold (structure, tooling, CI config)
- [x] Core scanner: walk directories for .git + .scripts
- [x] Shell script parser: extract commands from .sh files
- [x] CLI `scan` command with flags (depth, top, projects)
- [x] SQLite persistence (scans, projects, scripts, commands, tool_counts)
- [x] Stats command (query stored data, top tools, project breakdowns)
- [ ] Unit tests for scanner and parser

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

### Phase 4: Polish & Release [NOT STARTED]
- [ ] Unit tests (80%+ coverage)
- [ ] Integration tests for MCP server
- [ ] Performance tuning for large directory trees
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] v1.0.0 release

## Test Coverage
**Current:** 0.0%  |  **Target:** 80%

| Package | Coverage | Status |
|---------|----------|--------|
| cmd | 0.0% | No tests |
| internal/scanner | 0.0% | No tests |
| internal/store | 0.0% | No tests |
| internal/flow | 0.0% | No tests |
| internal/cmdlog | 0.0% | No tests |
| internal/mcp | 0.0% | No tests |
| internal/patterns | 0.0% | No tests |
| internal/deps | 0.0% | No tests |
