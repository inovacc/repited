# repited

Dual-mode binary: CLI tool + MCP server (stdio) for Claude Code. Reduces repeated multi-command workflows into single tool calls.

Scans `.scripts/` folders across projects, analyzes tool usage patterns, detects workflows, and runs full build→test→lint→commit pipelines in one shot.

## Installation

```bash
go install github.com/inovacc/repited@latest

# Register as Claude Code MCP server
repited mcp install --global --client claude
```

## Usage

```bash
# Scan directories for .scripts and analyze tool usage
repited scan D:/ --depth 5 --top 30

# Query stored scan data
repited stats --list
repited stats --projects

# Show command relationships and patterns
repited relations

# Run full development workflow (build→test→lint→stage→commit)
repited flow --message "feat: add new feature"
repited flow --dry-run         # preview what would run
repited flow --skip lint,test  # skip specific steps

# Manage workflow patterns and rules
repited patterns init          # create builtin patterns
repited patterns detect        # analyze scan data for new patterns
repited patterns suggest .     # suggest workflows for current project
repited patterns rules         # list all rules

# MCP server
repited mcp serve              # start stdio MCP server (used by Claude Code)
repited mcp install --global --client claude
repited mcp uninstall
```

## Commands

| Command | Description |
|---------|-------------|
| `scan` | Scan directories for .scripts folders, save to SQLite |
| `stats` | Query stored scan data (top tools, projects, aggregates) |
| `relations` | Show command co-occurrence, sequences, positions, clusters |
| `flow` | Run build→test→lint→stage→commit pipeline |
| `patterns` | Manage workflow patterns and rules (init, detect, list, suggest, rules) |
| `mcp` | MCP server management (serve, install, uninstall) |
| `version` | Print version information |
| `cmdtree` | Display command tree visualization |
| `aicontext` | Generate AI context documentation |

## MCP Server

When installed as a Claude Code MCP server, repited exposes 7 tools:

| Tool | Description |
|------|-------------|
| `flow` | Run full development workflow pipeline |
| `scan` | Scan directories for .scripts folders |
| `stats` | Query SQLite scan statistics |
| `relations` | Analyze command co-occurrence patterns |
| `patterns` | Manage workflow patterns and rules |
| `scout` | Browser automation and web interaction |
| `next-steps` | Suggest what to do after push/commit/merge/release |

## How It Works

1. Walks directory trees looking for folders containing both `.git` and `.scripts`
2. Reads all `.sh`/`.bash` scripts inside each `.scripts` directory
3. Parses scripts to extract tool/command invocations (handles pipelines, chains, multi-word commands)
4. Persists results to SQLite for cross-session analysis
5. Detects workflow patterns from command sequences and co-occurrence
6. Exposes everything as MCP tools for Claude Code integration

## Data Directory

All data stored in `%LOCALAPPDATA%\Repited\`:
- `repited.db` — SQLite database (scans, projects, scripts, commands)
- `commands/` — KSUID-named command log files (`{ksuid}_{command}.txt`)
- `patterns/` — Builtin and detected workflow patterns and rules

## Development

```bash
task build        # build with ldflags
task test         # tests with coverage
task lint:fix     # golangci-lint --fix
task pre-commit   # lint:fix + vet + build
task check        # fmt + vet + lint + test
```

### Pre-commit Hook

A git pre-commit hook automatically runs before every commit:
1. `task lint:fix` — auto-fix lint issues
2. `go vet ./...` — static analysis
3. `go build ./...` — compilation check

## Release

```bash
task release:snapshot    # snapshot release
git tag v1.0.0 && task release  # production release
```

## License

MIT
