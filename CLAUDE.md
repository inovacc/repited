# CLAUDE.md - repited

## Project Overview

Dual-mode binary: CLI tool + MCP server (stdio) for Claude Code.
Reduces repeated multi-command workflows into single tool calls.
Scans `.scripts/` folders, analyzes tool usage, detects patterns, and runs full build→test→lint→commit pipelines.

## Build & Test

```bash
task build           # build with ldflags
task test            # tests with coverage
task lint:fix        # golangci-lint --fix
task pre-commit      # lint:fix + vet + build (runs on git commit)
task check           # fmt + vet + lint + test
go run . scan .      # scan current dir
go run . flow --dry-run  # preview workflow
go run . mcp serve   # start MCP server (stdio)
go run . patterns init   # initialize patterns
```

## Pre-commit Rules

Git pre-commit hook runs automatically:
1. `task lint:fix` — golangci-lint with auto-fix
2. `go vet ./...` — static analysis
3. `go build ./...` — compilation check
4. Re-stages any auto-fixed files

## Architecture

- `cmd/` — Cobra commands (root, scan, stats, relations, flow, mcp, patterns, version, cmdtree, aicontext)
- `internal/scanner/` — Directory walker and shell script parser
- `internal/store/` — SQLite schema, migrations, queries (scans, projects, scripts, commands, tool_counts)
- `internal/flow/` — Pipeline engine with sequential steps and failure modes
- `internal/cmdlog/` — KSUID-named command log files
- `internal/mcp/` — MCP server: tool handlers, install/uninstall logic
- `internal/patterns/` — Pattern detection, builtin patterns/rules, project matching
- `docs/` — Project documentation

## Key Files

| File | Purpose |
|------|---------|
| `cmd/scan.go` | Scan directories for .scripts, save to SQLite |
| `cmd/flow.go` | Build→test→lint→stage→commit pipeline |
| `cmd/mcp.go` | MCP serve, install, uninstall |
| `cmd/patterns.go` | Pattern init, detect, list, suggest, rules |
| `cmd/patterns_edit.go` | Pattern enable/disable/edit commands |
| `cmd/stats.go` | Query stored scan data |
| `cmd/relations.go` | Command co-occurrence analysis |
| `internal/scanner/scanner.go` | Directory walker + command extractor |
| `internal/store/store.go` | SQLite schema and SaveScan() |
| `internal/store/query.go` | ListScans, TopTools, Stats queries |
| `internal/store/relations.go` | Sequences, co-occurrence, positions, clusters |
| `internal/flow/flow.go` | Pipeline engine (Step, Result, Pipeline) |
| `internal/cmdlog/cmdlog.go` | KSUID log files at AppData\Local\Repited\commands\ |
| `internal/mcp/server.go` | MCP server with 7 tools (flow, scan, stats, relations, patterns, scout, next-steps) + install/uninstall |
| `internal/patterns/patterns.go` | Pattern store, detection, builtin patterns & rules |

## MCP Server

Install: `go install github.com/inovacc/repited@latest && repited mcp install --global --client claude`

Tools exposed:
- `flow` — build→test→lint→stage→commit pipeline
- `scan` — scan directories for .scripts
- `stats` — query SQLite scan data
- `relations` — command co-occurrence patterns
- `patterns` — init/detect/list/suggest/rules

## Data Directory

`C:\Users\<user>\AppData\Local\Repited\`:
- `repited.db` — SQLite database
- `commands/` — `{ksuid}_{command}.txt` log files
- `patterns/` — `builtin-patterns.json`, `builtin-rules.json`, `detected-*.json`

## Conventions

- Parse `.sh`, `.bash`, and `.ps1` files (skip `.go`, `.py`, `.js`)
- Track multi-word commands: go, git, gh, docker, kubectl, task, terraform, npm, cargo, pip, omni
- Skip shell builtins (cd, export, echo, etc.)
- Skip code fragments (Go/Python/JS syntax)
- Commands must start with lowercase letter
- MCP server: logs to stderr (slog JSON), JSON-RPC on stdout
- Flow: test/lint failures are warnings, build/vet failures stop
