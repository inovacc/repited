# Known Issues

## Open Issues

_No critical issues._

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
| MCP server low test coverage | 49.9% with 13 integration tests via in-memory transports | 2026-03-09 |
| deps package coverage below target | 92.5% with mock injection helper | 2026-03-09 |
