# Known Issues

## Open Issues

### No git remote configured
- **Severity:** Medium
- **Description:** Project has no git remote set up. GitHub Actions workflows exist but won't run.
- **Workaround:** Development works fine locally. Set remote when ready to publish.

### Zero test coverage
- **Severity:** High
- **Description:** All 9 packages have 0% test coverage. Target is 80%.
- **Impact:** No regression protection for scanner, store, flow, MCP server, or patterns.

### TODO comments in aicontext.go
- **Severity:** Low
- **Description:** Lines 130 and 146 have generic TODO comments (`// TODO: customize for your app`)
- **Workaround:** Not blocking. AIContext command works but has scaffold-generated comments.

## Resolved Issues

| Issue | Resolution | Date |
|-------|------------|------|
| Parser noise (-, }, EOF as commands) | Added isShellSyntax(), isCodeFragment() filters | 2026-03-09 |
| Uppercase words detected as commands | Added lowercase-first-char requirement | 2026-03-09 |
| File paths detected as commands | Added / and extension filters | 2026-03-09 |
| go vet redundant newline in relations.go | Fixed fmt.Println("...\n") → fmt.Println("...") + fmt.Println() | 2026-03-09 |
| filepath import removed from scan.go | Re-added since filepath.Dir(dbPath) was still used | 2026-03-09 |
