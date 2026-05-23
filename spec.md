# Sensible Specification

## Use Case

Container signals host to execute execlineb scripts via shared filesystem.

## Architecture

```
Container                          Host
──────────                         ────
sensible-do "echo hello"      →   disk: ${SENSIBLE_TASKS_DIR}/pending/
                               ←   disk: ${SENSIBLE_TASKS_DIR}/done/
       ↑                              ↑
  reads result                    sensible-consume
                                    executes via execlineb
```

## Components

| Command | Purpose |
|---------|---------|
| `sensible-do` | Enqueue execlineb script(s), chains implicitly |
| `sensible-consume` | One-shot worker, processes all ready tasks |
| `sensible-status` | Check task result (JSON or specific field) |
| `sensible-list` | List pending tasks |

## Directory Structure

```
${SENSIBLE_TASKS_DIR}/
├── pending/
│   └── <timestamp>-script-<n>.json
└── done/
    └── <timestamp>-script-<n>.json
```

| Directory | Purpose |
|-----------|---------|
| `pending/` | Tasks waiting to execute (or waiting for parent) |
| `done/` | Completed tasks |

## Queue Item Format

```json
{
  "id": "script-1",
  "file_id": "2026-04-30T12:00:00.123456789Z-script-1",
  "request": "echo hello",
  "status": "queued|success|failed|timeout",
  "depends_on": "<parent-file_id>",  // optional
  "exit_code": 0,
  "stdout": "...",
  "stderr": "...",
  "duration_ms": 123,
  "timestamp": "2026-04-30T12:00:00.123456789Z"
}
```

## Task Status Flow

```
queued → success
       → failed
       → timeout
```

## Chaining

Multiple scripts on command line create implicit dependency chain:

```bash
sensible-do "compile" "test" "deploy"
```

Creates:
1. Task 1: `compile` (no dependency)
2. Task 2: `test` (depends on Task 1)
3. Task 3: `deploy` (depends on Task 2)

Each script runs only after its predecessor completes successfully.

## Worker Behavior

`sensible-consume` (one-shot, triggered by systemd):
1. Finds oldest ready task in `pending/`
2. Checks if parent complete (if `depends_on` set)
3. If parent not complete, skips
4. Executes task via execlineb
5. Moves to `done/`
6. Repeats until no ready tasks
7. Exits

## Configuration

Config file: `/etc/sensible/config.json` or `~/.config/sensible/config.json`

```json
{
  "whitelist": ["^echo", "^make"],
  "blacklist": ["^rm", "^dd"]
}
```

- **whitelist**: Regex patterns - allow matching scripts
- **blacklist**: Regex patterns - deny matching scripts
- Whitelist takes precedence over blacklist
- Empty whitelist = all allowed (unless blacklisted)

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SENSIBLE_TASKS_DIR` | `/var/lib/sensible/tasks` | Task storage |
| `SENSIBLE_CONFIG` | - | Config file path |

## Timestamp Format

RFC3339Nano: `2026-04-30T12:00:00.123456789Z`

Provides nanosecond resolution for sorting.