# Backlog

## Priority Levels

| Priority | Timeline |
|----------|----------|
| P1 | This sprint |
| P2 | This quarter |
| P3 | Future |

## Items

### P1 - This Sprint
- **Unit tests for scanner package** — Small — Test — 0% coverage
- **Unit tests for store package** — Small — Test — 0% coverage
- **Unit tests for flow package** — Small — Test — 0% coverage
- **Unit tests for patterns package** — Small — Test — 0% coverage
- **Unit tests for cmdlog package** — Small — Test — 0% coverage
- **Unit tests for deps package** — Small — Test — 0% coverage
- **Integration tests for MCP server** — Medium — Test — 0% coverage
- **TODO in cmd/aicontext.go:130,146** — Small — Tech Debt — Generic comments need project-specific customization

### P2 - This Quarter
- **GitHub remote setup** — Small — Infrastructure — No git remote configured
- **CI/CD pipeline** — Medium — Infrastructure — GitHub Actions workflows exist but no remote
- **JSON output format for scan/stats** — Small — Feature
- **Filter by tool name or pattern** — Small — Feature
- **Exclude patterns flag** — Small — Feature
- **Pattern editing CLI** — Medium — Feature — Enable/disable rules, modify patterns
- **Custom user-defined patterns** — Medium — Feature — Let users create their own workflow patterns

### P3 - Future
- **Interactive TUI mode** — Large — Feature
- **Watch mode (rescan on changes)** — Medium — Feature
- **PowerShell script parsing** — Medium — Feature
- **Python script parsing** — Medium — Feature
- **MCP project-level install** — Medium — Feature — Per-project .mcp.json config
- **Pattern sharing** — Large — Feature — Export/import patterns between teams

## Resolved

| Item | Resolution | Date |
|------|-----------|------|
| SQLite persistence | Implemented in v0.1.0 | 2026-03-09 |
| Command relations analysis | Implemented in v0.2.0 | 2026-03-09 |
| Flow pipeline | Implemented in v0.2.0 | 2026-03-09 |
| MCP server | Implemented in v0.3.0 | 2026-03-09 |
| Pattern detection | Implemented in v0.3.0 | 2026-03-09 |
| Pre-commit hook | Implemented in v0.3.0 | 2026-03-09 |
| Tool dependency graph | Implemented as relations command | 2026-03-09 |
