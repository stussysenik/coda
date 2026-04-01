# coda

![Demo](demo.gif)


**Close the loop. Claude Code stops when your system is healthy, not when it feels done.**

Coda is an autonomous dev orchestrator for [Claude Code](https://docs.anthropic.com/en/docs/claude-code). It turns Claude from an open-loop assistant — one that stops when *it thinks* it's done — into a closed-loop system that stops when your **server compiles, tests pass, and the gate clears**.

---

## The Problem

Without Coda, Claude Code operates in an **open loop**:

```
Developer → Claude Code → writes code → "I'm done!" → Developer checks → finds errors → re-prompts
```

The developer *is* the feedback loop. Every cycle through a human costs minutes.

With Coda, Claude Code operates in a **closed loop**:

```
Developer → Claude Code → writes code → tries to stop → Coda checks gate → tests failing?
    ↑                                                                            │
    └────────────────── Claude keeps working ←───────────────────────────────────┘
```

Coda observes your running dev server, your test suite, your file system — and **blocks Claude from stopping** until the system is actually healthy. No human in the loop. No re-prompting.

---

## Architecture Overview

```mermaid
flowchart TD
    CC["Claude Code"]

    subgraph Coda["coda (Go binary)"]
        MCP["MCP Server<br/><code>coda serve</code>"]
        SH["Stop Hook<br/><code>coda hook Stop</code>"]
        PTH["PostToolUse Hook<br/><code>coda hook PostToolUse</code>"]
        OM["Observer Manager"]
        LE["Lua Engine"]
        WP["Worktree Pool"]
        IPC["IPC Socket"]
    end

    CC -->|"stdio (MCP protocol)"| MCP
    CC -->|"hook event"| SH
    CC -->|"hook event"| PTH

    MCP --> OM
    MCP --> LE
    MCP --> WP

    SH -->|"Unix socket"| IPC
    PTH -->|"Unix socket"| IPC
    IPC --> OM
    IPC --> LE

    OM -->|"tails, file watchers"| EXT["Dev Server / Processes"]
    LE -->|"gate evaluation"| TS["Test Suite"]
    WP -->|"git worktree + claude -p"| GIT["Git Repository"]
```

---

## How It Works

### The Stop Hook Gate — The Money Diagram

This is the core mechanism. When Claude Code decides it's done, the Stop hook intercepts and asks Coda: "Is the system actually healthy?"

```mermaid
sequenceDiagram
    participant CC as Claude Code
    participant SH as coda hook Stop
    participant IPC as Unix Socket
    participant CS as coda serve
    participant LE as Lua Engine
    participant TS as Test Suite

    CC->>SH: "I want to stop" (stdin JSON)
    SH->>IPC: evaluate_gate
    IPC->>CS: route to Lua engine
    CS->>LE: EvaluateGate()
    LE->>TS: coda.run("mix test")
    TS-->>LE: exit_code=1 (failures)
    LE-->>CS: pass=false, reason="Tests failing"
    CS-->>IPC: {pass: false, reason: "..."}
    IPC-->>SH: gate FAIL

    Note over SH: exit code 2 = BLOCK THE STOP
    SH->>CC: stderr: "[coda/gate] Tests failing: ..."

    Note over CC: Claude sees the failure reason<br/>and continues working

    CC->>CC: fixes the code
    CC->>SH: "I want to stop" (attempt #2)
    SH->>IPC: evaluate_gate
    IPC->>CS: route to Lua engine
    CS->>LE: EvaluateGate()
    LE->>TS: coda.run("mix test")
    TS-->>LE: exit_code=0 (all pass)
    LE-->>CS: pass=true, reason="All checks pass"
    CS-->>IPC: {pass: true}
    IPC-->>SH: gate PASS

    Note over SH: exit code 0 = LET CLAUDE STOP
    SH-->>CC: (normal exit)
```

**Exit code 2** is the magic number. Claude Code interprets it as "the hook blocked the stop — here's why" and feeds `stderr` back to the model as context.

### PostToolUse Observation Injection

After every tool call Claude makes, Coda injects any pending observations (server errors, file changes) into Claude's context — without Claude having to ask.

```mermaid
sequenceDiagram
    participant CC as Claude Code
    participant PTH as coda hook PostToolUse
    participant IPC as Unix Socket
    participant CS as coda serve
    participant OM as Observer Manager
    participant T as Tail (phoenix)

    Note over T: Server emits CompileError
    T->>OM: line buffered in ring buffer

    CC->>CC: uses Edit tool (writes code)
    CC->>PTH: PostToolUse event (stdin JSON)
    PTH->>IPC: drain_observations
    IPC->>CS: route to observer manager
    CS->>OM: Drain()
    OM->>T: drain ring buffer
    T-->>OM: ["CompileError in lib/foo.ex:42"]
    OM-->>CS: observations
    CS-->>IPC: {observations: [...]}
    IPC-->>PTH: formatted lines
    PTH->>CC: stdout: "[coda/phoenix] CompileError..."

    Note over CC: Claude sees the error in its<br/>context and self-corrects
```

### Full Dev Session Lifecycle

```mermaid
sequenceDiagram
    participant D as Developer
    participant CC as Claude Code
    participant CS as coda serve (MCP)
    participant LE as Lua Engine
    participant OM as Observer Manager

    D->>CC: "Add user authentication"
    CC->>CS: set_mode("dev")
    CS->>LE: activate dev mode
    LE->>OM: start tail "mix phx.server"
    CS-->>CC: Mode activated + system prompt

    loop Development Cycle
        CC->>CC: writes/edits code
        Note over CC,OM: PostToolUse hook drains observations
        CC->>CS: check_gate (voluntary)
        CS->>LE: evaluate gate
        LE-->>CS: FAIL: "Server has errors"
        CS-->>CC: [gate:FAIL] Server has errors
        CC->>CC: fixes errors
    end

    CC->>CC: "I think I'm done"
    Note over CC,LE: Stop hook evaluates gate
    Note over LE: Tests pass, server clean
    CC-->>D: Task complete
```

---

## MCP Tools

Coda exposes 9 tools to Claude Code over MCP:

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `tail_server` | Start, stop, or read a tailed process | `action` (start/stop/read), `command`, `id` |
| `read_tail` | Get last N lines from a tailed process | `id`, `lines` (default 50) |
| `get_observations` | Drain all pending observations from all observers | — |
| `run_tests` | Execute a test command, return structured pass/fail | `command` (default `mix test`) |
| `check_gate` | Evaluate the active mode's completion gate | — |
| `set_mode` | Switch the active Lua mode | `mode` (e.g. `"dev"`) |
| `screenshot` | Take a CDP screenshot of a browser page | `url` (default `localhost:4000`) |
| `worktree_spawn` | Create a git worktree + launch parallel Claude session | `name`, `prompt` |
| `worktree_status` | Check status of all parallel worktree sessions | — |

---

## Lua Modes

Modes are the central configuration unit. A mode defines **what to observe**, **what to tell Claude**, and **when Claude can stop**.

```lua
-- dev.lua — Development mode for coda
local coda = require("coda")

coda.define_mode({
    name = "dev",

    -- Injected into Claude's context when mode activates
    system_prompt = [[You are in dev mode. After every code change:
1. Check the server tail for compilation errors
2. Run the test suite
3. Fix any failures before moving on

Never stop while the gate is failing. Use check_gate to verify.]],

    -- Which MCP tools this mode needs
    tools = {"tail_server", "read_tail", "run_tests", "check_gate", "get_observations"},

    -- Processes to observe (started automatically on mode activation)
    observers = {
        {type = "tail", command = "mix phx.server", id = "phoenix"},
    },

    -- The completion gate — must return (bool, string)
    gate = function()
        -- Check 1: Server tail for errors
        local output = coda.get_tail_output("phoenix")
        if output and (output:find("error") or output:find("Error")) then
            return false, "Server has errors:\n" .. output:sub(1, 500)
        end

        -- Check 2: Test suite
        local result = coda.run("mix test --no-color 2>&1")
        if result.exit_code ~= 0 then
            return false, "Tests failing:\n" .. result.output:sub(-500)
        end

        return true, "All checks pass"
    end,
})
```

### Mode Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Inactive: coda serve starts
    Inactive --> Active: set_mode("dev")

    state Active {
        [*] --> ObserversRunning: start tails + watchers
        ObserversRunning --> GateEvaluating: Stop hook fires
        GateEvaluating --> GateFailed: tests fail / server errors
        GateEvaluating --> GatePassed: all checks pass
        GateFailed --> ObserversRunning: Claude continues working
        GatePassed --> [*]: Claude stops
    }

    Active --> Inactive: set_mode(other) / serve exits
    Inactive --> [*]
```

---

## Observer System

Observers are background processes that feed data into Claude's context. They run continuously and buffer output in ring buffers that get drained on each `PostToolUse` hook.

```mermaid
flowchart LR
    subgraph Observers
        T1["Tail: phoenix<br/><code>mix phx.server</code>"]
        T2["Tail: tests<br/><code>mix test --watch</code>"]
        FW["File Watcher<br/><code>fsnotify</code>"]
    end

    subgraph Buffers["Ring Buffers (fixed-size, circular)"]
        R1["Ring 1<br/>1000 lines"]
        R2["Ring 2<br/>1000 lines"]
        R3["Ring 3<br/>events"]
    end

    subgraph Drain["Drain (PostToolUse)"]
        D["Observer Manager<br/><code>Drain()</code>"]
    end

    T1 -->|"stdout/stderr"| R1
    T2 -->|"stdout/stderr"| R2
    FW -->|"file events"| R3

    R1 --> D
    R2 --> D
    R3 --> D

    D -->|"formatted lines"| CC["Claude Code<br/>context"]
```

The ring buffer design means Coda never grows unbounded — old lines are overwritten, and only the most recent output matters.

---

## Worktrees

Coda can spawn parallel Claude Code sessions in isolated git worktrees. Each session works on its own branch without interfering with the main session.

```mermaid
flowchart TD
    CS["coda serve (main session)"]

    CS -->|"worktree_spawn('auth', 'Add login')"| W1
    CS -->|"worktree_spawn('api', 'Add REST endpoints')"| W2
    CS -->|"worktree_spawn('tests', 'Write integration tests')"| W3

    subgraph W1["Worktree: .worktrees/auth"]
        B1["branch: coda/auth"]
        C1["claude -p 'Add login'"]
    end

    subgraph W2["Worktree: .worktrees/api"]
        B2["branch: coda/api"]
        C2["claude -p 'Add REST endpoints'"]
    end

    subgraph W3["Worktree: .worktrees/tests"]
        B3["branch: coda/tests"]
        C3["claude -p 'Write integration tests'"]
    end

    CS -->|"worktree_status"| STATUS["[running] auth — Add login<br/>[completed] api — Add REST endpoints (2m34s)<br/>[running] tests — Write integration tests"]
```

---

## IPC Architecture

Hooks are **short-lived** processes (spawned per-event by Claude Code). The MCP server is a **long-lived** process (runs for the entire session). They communicate over a Unix domain socket.

```mermaid
flowchart LR
    subgraph ShortLived["Short-lived (per-event)"]
        SH["coda hook Stop"]
        PTH["coda hook PostToolUse"]
    end

    SOCK["Unix Socket<br/><code>~/.local/state/coda/coda.sock</code>"]

    subgraph LongLived["Long-lived (per-session)"]
        CS["coda serve<br/>(MCP server)"]
        OM["Observer Manager"]
        LE["Lua Engine"]
    end

    SH -->|"evaluate_gate"| SOCK
    PTH -->|"drain_observations"| SOCK
    SOCK --> CS
    CS --> OM
    CS --> LE
```

This split is necessary because Claude Code hooks are ephemeral — they start, do one thing, and exit. But observers need to run continuously. The IPC bridge connects the two worlds.

---

## Quick Start

```bash
# Build
cd ~/Desktop/coda
go build -o coda .

# Install into your project
cd /path/to/your/project
~/Desktop/coda/coda install
```

```
coda install — setting up Claude Code integration

1. Registering MCP server... OK
2. Installing hooks... OK
3. Installing /mode skill... OK
4. Copying default Lua scripts... OK

Done! Start Claude Code and use /mode dev to activate dev mode.
```

```bash
# Check status
coda status
```

```
coda serve: not running
  socket: ~/.local/state/coda/coda.sock (unreachable)
```

<details>
<summary>Terminal screenshots</summary>

<br/>

![coda help](docs/screenshots/coda-help.png)

![coda status](docs/screenshots/coda-status.png)

</details>

Once Claude Code starts and connects:

```
coda serve: running
  socket: ~/.local/state/coda/coda.sock
  active mode: dev
  available modes: dev
  active tails: phoenix
  gate: PASS — All checks pass
```

### CLI Reference

```
coda — Claude Code Autonomous Dev Orchestrator

Usage:
  coda serve       Run as MCP server (stdio)
  coda hook <evt>  Handle a Claude Code hook event (Stop, PostToolUse)
  coda mode <cmd>  Manage modes (set, list)
  coda install     One-command project setup
  coda status      Show current state (observers, gate, mode)
```

---

## Project Structure

```
coda/
├── main.go                  # CLI entrypoint — routes subcommands
├── go.mod                   # Go 1.24, MCP SDK v0.2.0, gopher-lua v1.1.1
│
├── cmd/                     # Subcommand implementations
│   ├── serve.go             # Start MCP server + IPC listener
│   ├── hook.go              # Dispatch hook events (Stop, PostToolUse)
│   ├── install.go           # One-command project setup
│   ├── mode.go              # Manage modes (set, list)
│   └── status.go            # Show current state via IPC
│
├── mcp/                     # MCP server + tool registration
│   ├── server.go            # Server struct, wires observer/lua/worktree
│   └── tools.go             # 9 MCP tools (tail, test, gate, screenshot, worktree)
│
├── hooks/                   # Claude Code hook handlers (short-lived)
│   ├── dispatch.go          # Hook input parsing (stdin JSON)
│   ├── stop.go              # THE gate — exit 2 blocks Claude from stopping
│   ├── posttool.go          # Drain observations after every tool use
│   ├── ipc.go               # Unix socket client for hook→serve comms
│   └── install.go           # Patch .claude/settings.json with hook config
│
├── observer/                # Background observation system
│   ├── manager.go           # Coordinates tails, watchers; drain interface
│   ├── tail.go              # Process tailer (shell command → ring buffer)
│   ├── ring.go              # Thread-safe circular buffer
│   ├── ring_test.go         # Ring buffer tests
│   ├── filewatch.go         # fsnotify file watcher
│   └── screenshot.go        # CDP screenshot via chromedp
│
├── lua/                     # Lua scripting engine
│   ├── engine.go            # VM management, coda module, gate evaluation
│   ├── engine_test.go       # Engine tests
│   ├── mode.go              # Mode struct and config types
│   └── recipe.go            # Recipe execution (planned)
│
├── worktree/                # Parallel session management
│   ├── worktree.go          # Git worktree create/remove
│   ├── pool.go              # Session pool (spawn, status, cleanup)
│   └── session.go           # Individual Claude session (claude -p)
│
├── lua.d/                   # Bundled Lua scripts (copied on install)
│   ├── modes/
│   │   └── dev.lua          # Default dev mode
│   └── recipes/
│       └── test_and_fix.lua # Test-and-fix recipe
│
└── config/                  # XDG path resolution
    └── paths.go             # ~/.config/coda, ~/.local/state/coda
```

---

## Configuration

### XDG Paths

| Path | Purpose |
|------|---------|
| `~/.config/coda/` | User configuration root |
| `~/.config/coda/lua/modes/` | Mode scripts (e.g. `dev.lua`) |
| `~/.config/coda/lua/recipes/` | Recipe scripts |
| `~/.local/state/coda/` | Runtime state |
| `~/.local/state/coda/coda.sock` | IPC Unix domain socket |

### Lua API

| Function | Description |
|----------|-------------|
| `coda.define_mode({...})` | Register a mode with name, system_prompt, tools, observers, gate |
| `coda.get_tail_output(id)` | Read all buffered output from a named tail |
| `coda.run(command)` | Run a shell command; returns `{exit_code, output}` |

### Mode Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Mode identifier (e.g. `"dev"`) |
| `system_prompt` | string | Injected into Claude's context on activation |
| `tools` | string[] | MCP tools this mode needs |
| `observers` | table[] | Processes/files to observe (`{type, command, id}`) |
| `gate` | function | Returns `(bool, string)` — pass/fail + reason |

---

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | v0.2.0 | MCP server (stdio transport) |
| [yuin/gopher-lua](https://github.com/yuin/gopher-lua) | v1.1.1 | Embedded Lua 5.1 VM |
| [chromedp/chromedp](https://github.com/chromedp/chromedp) | v0.14.2 | CDP browser automation (screenshots) |
| [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) | v1.9.0 | File system event watcher |

---

## C4 Context Diagram

![Coda System Context Diagram](https://kroki.io/c4plantuml/svg/eJxdU8GS2jAMvfMVKic6hXDpqadlQ2fLzG6Hge70uGMcNXFxrIylBLj1I_qF_ZLKCbDLnhxb0ntPT8odi4nS1n70wQXr2wKhEmn4y3wezSErnVTtrmWMloJgkMxSPW-8CalmxlJ4t5vnn2fr9PT89DivDQtGfXrJU8VRsiahj8SJR8ipMPDvz1_YnjSthnMOLJ0po6lHozVGpjApsJvCeIkdemowjvXyzMiQe5MkKgzCL4qwWM0Ms1OsAoohu1aV44-jgWBi-wItf1OZ0BYrsFS4UMJQr-ohf1zB7gSLIFWkxtk3KCo7YejRF7dCgWpqOZECRVshSzRCMYOnfA3qV4cRPkFFtGeQyghYT4z6qcIRi52xe_BETaYsZ5qXr0eZqOPK8OAENtgQO8U8Jc4ttdFiEo2Ke6C4l4gIu2hCYr9q7UEG_sFB2PaXhIFZmcG6IgzuOIXvaTi_uR-HGOfVwmQpxkjxHZ5oe6wAP_SEbesEr3C1O0IKT6E5pbOHK40gYGd8a8RRuAXbRTpwr-5--EpY-VJds9pR4IqEoXMGrI6hxqJJDm3QD0txHeiD63QhxPA-iU3xS-g8rTQIIfKsfn1Lc7ik9eHB558XH2sTTInn3XnNuhq5jmSRuTdKt-Ym6eLOpg18uZybV4XJi5v01_7fNa1ZdxgK_V3-A1x3O6M=)

<details>
<summary>PlantUML source</summary>

```plantuml
@startuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Context.puml

title Coda — System Context Diagram

Person(dev, "Developer", "Uses Claude Code for AI-assisted development")
System(claude, "Claude Code", "AI coding assistant CLI by Anthropic")
System(coda, "Coda", "Autonomous dev orchestrator. MCP server + hooks that close the feedback loop.")

System_Ext(git, "Git Repository", "Source code + worktree branches")
System_Ext(server, "Dev Server", "e.g. Phoenix, Next.js — tailed for errors")
System_Ext(tests, "Test Suite", "e.g. mix test, pytest — gate evaluation")
System_Ext(browser, "Browser", "CDP screenshots via chromedp")

Rel(dev, claude, "Gives tasks")
Rel(claude, coda, "MCP tools + Hooks")
Rel(coda, git, "Worktree management")
Rel(coda, server, "Process tailing")
Rel(coda, tests, "Runs tests, evaluates gate")
Rel(coda, browser, "CDP screenshots")
@enduml
```

</details>

---

## License

Private — not yet published.
