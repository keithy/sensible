# AGENTS.md — Sensible

Remote execution daemon for AI agents, written in Go. Executes whitelisted actions via `execlineb` (not shell) to prevent command injection.

## Building

```bash
# Local queue interface (talks to disk directly)
go build -o sensible-queue ./cmd/sensible-queue

# HTTP server
go build -o sensible-server ./cmd/sensible-server

# HTTP client (talks to sensible-server)
go build -o sensible-client ./cmd/sensible-client

# Library (used by all above)
go build ./pkg/sensible
```

## Running

```bash
# Go CLI (local queue interface, talks to disk directly)
./sensible-queue do <action>        # Execute action directly
./sensible-queue worker             # Run background worker (continuous)
./sensible-queue list               # List pending tasks
./sensible-queue status <file_id>    # Check task status

# Bash CLI (same as Go, alternative implementation)
./sensible-queue.sh do <action>
./sensible-queue.sh worker
./sensible-queue.sh list
./sensible-queue.sh status <file_id>

# HTTP server
./sensible-server                   # Starts HTTP daemon on :8080

# HTTP client (Go/Bash, talks to sensible-server)
./sensible-client do <action>
./sensible-client.sh do <action>    # Bash version
```

## Testing

```bash
go test ./...
```

## Directory Structure

```
sensible/
├── pkg/sensible/           # Library
│   ├── task.go            # Task struct, interfaces
│   ├── storage.go         # Disk storage implementation
│   ├── executor.go        # execlineb execution
│   └── config.go          # Config loading
│
├── cmd/sensible-queue/   # Go CLI (local queue interface)
│   └── main.go
│
├── cmd/sensible-queue.sh  # Bash CLI (same functionality)
│
├── cmd/sensible-server/   # HTTP server
│   └── main.go
│
├── cmd/sensible-client/   # Go HTTP client
│   └── main.go
│
├── cmd/sensible-client.sh # Bash HTTP client

└── actions/               # Empty — execline scripts on deployed hosts
```

## Architecture

**Library (`pkg/sensible`) has no knowledge of HTTP or CLI — pure domain logic.**

```
CLI or HTTP Server
    ↓
pkg/sensible (library)
    ├── Task struct
    ├── TaskRepository interface (Save, Load, ListPending, MoveToDone, Delete)
    ├── Executor interface (Execute)
    └── Config struct
    ↓
Disk storage (pending/, done/)
```

**Decoupled layers:**
- `pkg/sensible/` — domain logic, interfaces, no external dependencies except stdlib
- `cmd/sensible/` — thin CLI wrapper around library
- `cmd/sensible-server/` — thin HTTP wrapper around library

**Pure disk storage — no in-memory state. No race conditions.**

## Directory Structure (Tasks)

```
${SENSIBLE_TASKS_DIR}/
├── pending/
│   ├── 2026-04-30T12:00:00.123456789Z-compile.json
│   └── 2026-04-30T12:00:01.456789012Z-test.json
└── done/
    ├── 2026-04-30T11:00:00.789012345Z-compile.json
    └── 2026-04-30T11:05:00.012345678Z-test.json
```

| Directory | Purpose |
|-----------|---------|
| `pending/` | Tasks waiting to execute (or waiting for parent) |
| `done/` | Completed tasks (success or failed) |

## Task Struct

```go
type Task struct {
    ID         string `json:"id"`           // Action name, e.g. "compile"
    FileID     string `json:"file_id"`      // Unique: "2026-04-30T12:00:00.123Z-compile"
    Request    string `json:"request,omitempty"`
    Status     string `json:"status"`       // queued, success, failed, timeout
    DependsOn  string `json:"depends_on,omitempty"` // FileID of parent task
    ExitCode   int    `json:"exit_code,omitempty"`
    Stdout     string `json:"stdout,omitempty"`
    Stderr     string `json:"stderr,omitempty"`
    DurationMs int64  `json:"duration_ms,omitempty"`
    Timestamp  string `json:"timestamp"`    // RFC3339Nano
}
```

## Interfaces

```go
type TaskRepository interface {
    Save(task *Task) error
    Load(id string) (*Task, error)
    ListPending() ([]*Task, error)
    MoveToDone(task *Task) error
    Delete(fileID string) error
}

type Executor interface {
    Execute(req string, timeout int) *Result
}

type Result struct {
    Status     string
    ExitCode   int
    Stdout     string
    Stderr     string
    DurationMs int64
}
```

## Configuration

| Env Var | Default |
|---------|---------|
| `SENSIBLE_PORT` | 8080 |
| `SENSIBLE_ACTIONS_DIR` | `/var/lib/sensible/actions` |
| `SENSIBLE_KEYS_DIR` | `/etc/sensible/keys` |
| `SENSIBLE_TASKS_DIR` | `/var/lib/sensible/tasks` |

**API Keys**: From `$(SENSIBLE_KEYS_DIR)/*.pem`. Empty dir = auth bypassed.

**Whitelist** (hardcoded):
```go
{Name: "status", Timeout: 10},
{Name: "restart", Timeout: 60},
{Name: "compile", Timeout: 600},
{Name: "update", Timeout: 300},
{Name: "test", Timeout: 300},
```

## HTTP API (sensible-server)

| Route | Method | Description |
|-------|--------|-------------|
| `/sensible` | GET/POST | Execute action |
| `/sensible` | GET/POST | `?field=<field>` extracts single field |
| `/sensible/:id` | GET | Poll task result |
| `/sensible/:id/` | POST | Chain new task to `:id` (async) |

**Sync vs Async**: Tasks ≤15s run sync (200). Longer return 202 and run in background.

## CLI Commands

```bash
sensible do <action> [args...]  # Execute action directly
sensible worker                  # Run as background worker (continuous)
sensible list                    # List pending tasks
sensible status <file_id>        # Check task status by FileID
```

## Gotchas

- **execlineb, not shell**: Actions via `execlineb <script>` — `$VAR`, `;`, `&&` don't work. Use `import -env VAR`.
- **Request parsing**: First token = action name, remainder = arguments.
- **Field extraction**: `?field=` returns raw text (not JSON).
- **Task FileID format**: `<timestamp>-<action>` (RFC3339Nano + action name).
- **No action scripts in repo**: `actions/` is empty — deployed hosts have execline scripts.
- **NetBird transport**: Sensible uses plain HTTP; NetBird's WireGuard overlay provides encryption.
- **Worker vs Executor**: `sensible worker` runs continuously. `./sensible-server` handles async tasks via goroutines.