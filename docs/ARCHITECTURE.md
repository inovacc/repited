# Architecture

## System Overview

```mermaid
flowchart TB
    User([User]) --> CLI[repited CLI]
    Claude([Claude Code]) --> MCP[MCP Server<br/>stdio transport]

    subgraph cmd["cmd/ — Cobra Commands"]
        CLI --> Root[root.go]
        Root --> Scan[scan.go]
        Root --> Stats[stats.go]
        Root --> Relations[relations.go]
        Root --> Flow[flow.go]
        Root --> MCPCmd[mcp.go]
        Root --> Patterns[patterns.go]
        Root --> Version[version.go]
        Root --> CmdTree[cmdtree.go]
        Root --> AIContext[aicontext.go]
    end

    subgraph mcp_pkg["internal/mcp/ — MCP Server"]
        MCP --> FlowTool[flow tool]
        MCP --> ScanTool[scan tool]
        MCP --> StatsTool[stats tool]
        MCP --> RelTool[relations tool]
        MCP --> PatTool[patterns tool]
        MCP --> ScoutTool[scout tool]
        MCP --> NextTool[next-steps tool]
    end

    subgraph core["internal/ — Core Logic"]
        Scan --> Scanner[scanner.Scan]
        ScanTool --> Scanner
        Scanner --> Walker[Directory Walker]
        Scanner --> Parser[Script Parser]

        Stats --> Store[store.Open]
        StatsTool --> Store
        Relations --> StoreRel[store relations]
        RelTool --> StoreRel
        Store --> SQLite[(SQLite DB)]
        StoreRel --> SQLite

        Flow --> Pipeline[flow.Pipeline]
        FlowTool --> Pipeline

        Patterns --> PatStore[patterns.PatternStore]
        PatTool --> PatStore
        PatStore --> PatDir[(patterns/ dir)]

        ScoutTool --> Deps[deps.EnsureInstalled]
        Deps --> OmniBin[omni binary]
        Deps --> ScoutBin[scout binary]
    end

    subgraph data["AppData\\Local\\Repited\\"]
        SQLite
        CmdLog[commands/<br/>KSUID logs]
        PatDir
    end

    Pipeline --> CmdLog
    Scan --> CmdLog
```

## MCP Server Architecture

```mermaid
sequenceDiagram
    participant CC as Claude Code
    participant T as StdioTransport
    participant S as MCP Server
    participant H as Tool Handlers
    participant DB as SQLite
    participant FS as Filesystem

    CC->>T: JSON-RPC request (tools/call)
    T->>S: Route to tool handler

    alt flow tool
        S->>H: registerFlowTool
        H->>FS: Detect project type (go.mod, package.json, Cargo.toml)
        H->>H: Build pipeline steps
        H->>H: Run pipeline (quiet mode)
        H->>FS: Save log to commands/
        H-->>S: CallToolResult (summary)
    end

    alt scan tool
        S->>H: registerScanTool
        H->>FS: Walk directories
        H->>DB: SaveScan()
        H-->>S: CallToolResult (stats)
    end

    alt patterns tool
        S->>H: registerPatternsTool
        H->>DB: Query sequences, positions, clusters
        H->>FS: Load/save pattern files
        H-->>S: CallToolResult (patterns)
    end

    alt next-steps tool
        S->>H: registerNextStepsTool
        H->>FS: Check project markers
        H->>FS: Load patterns
        H-->>S: CallToolResult (suggestions)
    end

    S->>T: JSON-RPC response
    T->>CC: Tool result

    Note over S: Logs go to stderr (slog JSON)
    Note over T: JSON-RPC on stdout
```

## Scan Flow

```mermaid
sequenceDiagram
    participant U as User
    participant C as scan command
    participant S as scanner.Scan
    participant DB as SQLite
    participant FS as Filesystem

    U->>C: repited scan /path --depth 3 --top 20
    C->>S: Scan(dir, maxDepth)

    loop Walk directory tree
        S->>FS: WalkDir (skip hidden, enforce depth)
        FS-->>S: Directory entry
        S->>FS: Check .git exists?
        S->>FS: Check .scripts exists?

        alt Both exist
            S->>FS: ReadDir(.scripts)
            FS-->>S: Script file list

            loop Each .sh file
                S->>S: extractCommands(path)
                S->>S: splitStatements (&&, ||, ;, |)
                S->>S: extractTool (identify command name)
                S->>S: Aggregate in toolFreq map
            end

            S->>S: SkipDir (don't descend further)
        end
    end

    S-->>C: ScanResult{Projects, ToolCounts}
    C->>DB: SaveScan (transaction with prepared stmts)
    C->>U: Print ranked tool table with bars
```

## Flow Pipeline

```mermaid
sequenceDiagram
    participant U as User/MCP
    participant F as flow command
    participant P as Pipeline
    participant E as exec.Command
    participant L as cmdlog

    U->>F: flow --message "feat: ..." --skip lint
    F->>F: Detect project type
    F->>P: NewPipeline(dir)

    alt Go project
        P->>E: go mod tidy
        E-->>P: ok
        P->>E: go build ./...
        E-->>P: ok
        P->>E: go vet ./...
        E-->>P: ok
        P->>E: go test -count=1 ./...
        E-->>P: ok/warned (OnFail=warn)
        P->>E: golangci-lint run ./...
        E-->>P: skipped
    end

    P->>E: git add .
    E-->>P: ok
    P->>E: git status --short
    E-->>P: ok
    P->>E: git commit -m "feat: ..."
    E-->>P: ok

    P-->>F: Results[]
    F->>L: Save log (KSUID_flow.txt)
    F->>U: Summary (passed/warned/skipped/failed)
```

## Command Parser Pipeline

```mermaid
flowchart LR
    Line[Script Line] --> Skip{Skip?}
    Skip -->|Comment/Shebang| Discard[Discard]
    Skip -->|Shell Syntax| Discard
    Skip -->|Code Fragment| Discard
    Skip -->|Valid| Split[splitStatements]

    Split --> S1["stmt 1"]
    Split --> S2["stmt 2"]
    Split --> SN["stmt N"]

    S1 --> Extract[extractTool]
    S2 --> Extract
    SN --> Extract

    Extract --> StripVars[Strip VAR=val prefixes]
    StripVars --> MultiWord{Multi-word tool?}
    MultiWord -->|"go, git, gh, etc."| MW["tool subcmd"]
    MultiWord -->|"omni subcmd"| Omni["omni subcmd"]
    MultiWord -->|Single word| Validate[isValidCommand]

    MW --> Freq[toolFreq map]
    Omni --> Freq
    Validate -->|Valid| Freq
    Validate -->|Builtin/Invalid| Discard
```

## Pattern Detection

```mermaid
flowchart TB
    DB[(SQLite)] --> Seqs[ToolSequences<br/>A→B pairs]
    DB --> Pos[ToolPositions<br/>first/middle/last]
    DB --> Clust[ToolClusters<br/>by category]

    Seqs --> FlowDet[detectFlowPatterns<br/>greedy chain following]
    Seqs --> GuardDet[detectGuardPatterns<br/>prerequisite pairs]
    Pos --> TearDet[detectTeardownPatterns<br/>script closers]
    Clust --> ClustDet[detectClusterPatterns<br/>co-occurring toolkits]

    FlowDet --> Detected[detected-YYYYMMDD.json]
    GuardDet --> Detected
    TearDet --> Detected
    ClustDet --> Detected

    Builtin[builtin-patterns.json<br/>10 workflows] --> PatStore[PatternStore]
    BuiltinR[builtin-rules.json<br/>12 rules] --> PatStore
    Detected --> PatStore

    PatStore --> Suggest[SuggestFlows<br/>match project markers]
```

## Data Model

```mermaid
erDiagram
    scans ||--o{ projects : contains
    scans ||--o{ tool_counts : aggregates
    projects ||--o{ scripts : contains
    scripts ||--o{ commands : contains

    scans {
        int id PK
        text root_dir
        text scanned_at
        int project_count
        int tool_count
    }
    projects {
        int id PK
        int scan_id FK
        text path
        int script_count
    }
    scripts {
        int id PK
        int project_id FK
        text name
        text path
        int command_count
    }
    commands {
        int id PK
        int script_id FK
        text tool
    }
    tool_counts {
        int id PK
        int scan_id FK
        text tool
        int count
    }
```
