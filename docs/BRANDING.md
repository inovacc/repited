# repited — Branding Names

## Project Identity

- **Current Name:** repited
- **Purpose:** CLI tool that scans project directories for `.scripts` folders and ranks the most frequently used tools and commands across your workspace
- **Domain:** Developer tooling, script analysis, workspace introspection
- **Audience:** Developers who use ephemeral `.scripts/` folders as command journals
- **Tech Stack:** Go, Cobra, SQLite

---

## Project Name Candidates

| Name | Style | Rationale |
|------|-------|-----------|
| **repited** | Current | Evokes "repeated" — finding patterns in repetition |
| **scriptor** | Descriptive | Latin for "writer/scribe" — reads and catalogs scripts |
| **tally** | Metaphorical | Counting tool occurrences, keeping a tally |
| **shellrank** | Compound | Ranks shell commands — says exactly what it does |
| **footprint** | Metaphorical | Traces the tools you leave behind in scripts |
| **recap** | Abstract | Summarizes your tooling habits across projects |
| **habito** | Abstract | Spanish/Latin for "habit" — discovers your tool habits |
| **cmdcensus** | Compound | Census of commands across your workspace |
| **tracery** | Metaphorical | Tracing patterns — also an ornamental term for interlocking lines |
| **repcli** | Portmanteau | "rep" (repetition) + "cli" — repeated CLI patterns |
| **drift** | Abstract | Short, brandable — how tool usage drifts across projects |
| **grooves** | Metaphorical | The well-worn paths in your workflow |

**Recommended:** `repited` (current) — short, unique, hints at "repeated patterns"

---

## Feature Names

| Feature | Current Name | Branded Name Options |
|---------|-------------|---------------------|
| Directory scanner | `scanner.Scan` | **Sweep**, **Traverse**, **Patrol** |
| Script parser | `extractCommands` | **Dissect**, **Distill**, **Sift** |
| Tool ranking | `ToolCount` / table output | **Leaderboard**, **Tally**, **Rank** |
| SQLite persistence | `store.SaveScan` | **Vault**, **Ledger**, **Archive** |
| Stats query | `stats` command | **Recall**, **Insight**, **Report** |
| Project discovery | `.git` + `.scripts` check | **Detect**, **Discover**, **Locate** |

---

## Component Names

| Component | Branded Name Options |
|-----------|---------------------|
| Scanner package | **Patrol**, **Sweep**, **Recon** |
| Store package | **Vault**, **Ledger**, **Stash** |
| Query layer | **Lens**, **Insight**, **Recall** |
| Command parser | **Sieve**, **Distill**, **Filter** |
| Bar chart output | **Gauge**, **Meter**, **Spark** |

---

## Taglines

| # | Tagline | Style |
|---|---------|-------|
| 1 | **Know your tools.** | Short & punchy |
| 2 | **What do your scripts actually run?** | Provocative |
| 3 | **Discover the tools you rely on most.** | Descriptive |
| 4 | **Your workspace, quantified.** | Aspirational |
| 5 | **Scan. Rank. Understand.** | Action-driven |
| 6 | **Script archaeology for developers.** | Technical |
| 7 | **Every command tells a story.** | Aspirational |
| 8 | **Find the patterns in your workflow.** | Descriptive |

**Recommended:** "Know your tools." — minimal, direct, memorable

---

## CLI Branding Themes

### Theme 1: Reconnaissance (military/exploration)

```
repited recon D:/          # scan directories
repited intel              # show stats from database
repited intel --debrief    # list all scans
repited intel --targets    # per-project breakdown
```

### Theme 2: Archaeology (digging through layers)

```
repited dig D:/            # scan directories
repited findings           # show stats from database
repited findings --sites   # per-project breakdown
repited findings --catalog # list all scans
```

### Theme 3: Minimal (current — clean verbs)

```
repited scan D:/           # scan directories
repited stats              # show stats from database
repited stats --list       # list all scans
repited stats --projects   # per-project breakdown
```

**Recommended:** Theme 3 (current) — clear, idiomatic, no learning curve

---

## Color Palette Suggestions

| Role | Color | Hex | Rationale |
|------|-------|-----|-----------|
| **Primary** | Deep Indigo | `#3B3F8C` | Technical authority, depth |
| **Secondary** | Slate Teal | `#3D8B8B` | Calm, analytical |
| **Accent** | Electric Amber | `#F5A623` | Highlights, discoveries, rankings |
| **Warning** | Coral Red | `#E74C3C` | Errors, missing data |
| **Muted** | Cool Gray | `#8E99A4` | Secondary text, borders, bars |

---

## Logo Concepts

1. **Stacked Bar Chart** — A stylized ascending bar chart made of terminal block characters (█), representing the ranked tool output. Captures the core output at a glance.

2. **Magnifying Glass over Script** — A magnifying lens hovering over a `#!/bin/bash` shebang line, symbolizing script inspection and discovery.

3. **Fingerprint made of Commands** — A thumbprint pattern where the ridges are formed by tiny command names (`git`, `go`, `curl`), representing unique project identity through tool usage.

4. **Radar Sweep** — A circular radar/sonar display with blips at different distances, representing the scan-and-discover nature of the tool. Ties to the "scan" command.

**Icon generation available via:**
```bash
iconforge forge --generate \
  --name repited \
  --primary "#3B3F8C" \
  --secondary "#3D8B8B" \
  --accent "#F5A623" \
  --output build/icons
```
