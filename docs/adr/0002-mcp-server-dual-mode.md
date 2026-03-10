# ADR-0002: MCP Server Dual-Mode Binary

## Status
Accepted

## Context
repited was built as a CLI tool to scan and analyze Claude Code's `.scripts/` folders. The natural next step was to make those capabilities directly available to Claude Code as MCP tools, eliminating the need for Claude Code to generate multi-step shell scripts for common workflows.

## Decision
- **Dual-mode binary:** Same `repited` binary serves as both CLI and MCP server
- **MCP transport:** stdio (stdout = JSON-RPC, stderr = logs via slog)
- **MCP SDK:** `github.com/modelcontextprotocol/go-sdk v1.4.0`
- **Install mechanism:** `repited mcp install --global --client claude` writes config to `~/.claude.json`
- **Tools exposed:** flow, scan, stats, relations, patterns, scout, next-steps
- **Dependency management:** Auto-install omni and scout via `go install` if not found

## Consequences

### Positive
- Claude Code can call repited tools directly instead of generating scripts
- Single binary simplifies installation (`go install` + `repited mcp install`)
- CLI commands remain available for manual use and debugging
- Typed MCP inputs provide validation and documentation
- Auto-install of dependencies reduces setup friction

### Negative
- MCP server mode adds ~3 dependencies (MCP SDK, jsonschema, uritemplate)
- stdio transport means only one Claude Code session can connect at a time
- Tool handlers duplicate some logic from cmd/ (necessary for quiet/non-CLI output)
