# Backlog

## Priority Levels

| Priority | Timeline |
|----------|----------|
| P1 | This sprint |
| P2 | This quarter |
| P3 | Future |

## Items

### P1 - This Sprint
- **v1.0.0 release with goreleaser** — Small — Infrastructure — Tag and release

### P2 - This Quarter
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
| Unit tests for scanner | 92.4% coverage | 2026-03-09 |
| Unit tests for store | 76.1% coverage | 2026-03-09 |
| Unit tests for flow | 88.2% coverage | 2026-03-09 |
| Unit tests for patterns | 79.3% coverage | 2026-03-09 |
| Unit tests for cmdlog | 92.5% coverage | 2026-03-09 |
| Unit tests for deps | 55.0% coverage | 2026-03-09 |
| TODO in cmd/aicontext.go | Replaced with repited-specific content | 2026-03-09 |
| GitHub remote setup | Published to github.com/inovacc/repited | 2026-03-09 |
| CI/CD pipeline | GitHub Actions workflows active | 2026-03-09 |
| Exclude patterns flag | Added --exclude flag and DefaultExcludes | 2026-03-09 |
| Integration tests for MCP server | 49.9% coverage, 13 integration tests | 2026-03-09 |
| Improve deps package coverage | 92.5% coverage | 2026-03-09 |
| JSON output format for scan/stats | Added --json flag | 2026-03-09 |
| Filter by tool name or pattern | Added --filter flag to stats/relations | 2026-03-09 |
| MCP project-level install | Added --project flag for .mcp.json | 2026-03-09 |
