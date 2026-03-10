# Known Issues

## Open Issues

### MCP server low test coverage
- **Severity:** Medium
- **Description:** MCP server package has only 10.6% coverage. Only helper functions are tested, not tool handlers.
- **Impact:** No regression protection for the 7 MCP tool handlers.

### deps package coverage below target
- **Severity:** Low
- **Description:** deps package at 55.0% coverage. Install() and EnsureAll() are hard to test without running real installs.
- **Workaround:** Core detection logic is tested. Install paths are simple exec wrappers.

## Resolved Issues

| Issue | Resolution | Date |
|-------|------------|------|
| Parser noise (-, }, EOF as commands) | Added isShellSyntax(), isCodeFragment() filters | 2026-03-09 |
| Uppercase words detected as commands | Added lowercase-first-char requirement | 2026-03-09 |
| File paths detected as commands | Added / and extension filters | 2026-03-09 |
| go vet redundant newline in relations.go | Fixed fmt.Println("...\n") → fmt.Println("...") + fmt.Println() | 2026-03-09 |
| filepath import removed from scan.go | Re-added since filepath.Dir(dbPath) was still used | 2026-03-09 |
| No git remote configured | Created repo at github.com/inovacc/repited | 2026-03-09 |
| Zero test coverage | Tests written for all 7 internal packages (~70% overall) | 2026-03-09 |
| TODO comments in aicontext.go | Replaced with repited-specific categories and structure | 2026-03-09 |
