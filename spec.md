# Sensible Specification

## Use Case

Container signals host to execute actions via shared filesystem.

## Components

```
Container                          Host
──────────                         ────
sensible-do "execlineb script"     sensible-worker
  │  (writes pending/)                │  (processes queue)
  └──────────────────────────────────┘
                    │
                    ▼
          ${SENSIBLE_TASKS_DIR}/
```

## Directory Structure

```
${SENSIBLE_TASKS_DIR}/
├── pending/
│   └── <timestamp>-<taskid>.json
└── done/
    └── <timestamp>-<taskid>.json
```

| Directory | Purpose |
|-----------|---------|
| `pending/` | Tasks waiting to execute (or waiting for parent) |
| `done/` | Completed tasks |

## Queue Item Format

```json
{
  "id": "<taskid>",
  "request": "execlineb action script",
  "status": "queued|success|failed|timeout",
  "depends_on": "<parent-taskid>",  // optional
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

Tasks can depend on parent completing first:

1. Task B has `depends_on: "task-A-id"`
2. Worker checks parent status before executing
3. Parent completes → triggerDependents("task-A-id")
4. Task B executes

## Worker Behavior

`sensible-worker`:
1. Finds oldest task in `pending/`
2. Checks if parent complete (if `depends_on` set)
3. If parent not complete, skips (returns nil, worker will retry later)
4. Executes task
5. Moves to `done/`
6. Triggers any dependents
7. Repeats until `pending/` empty
8. Waits for filesystem change (inotify) or polls

## Timestamp Format

RFC3339Nano: `2026-04-30T12:00:00.123456789Z`

Provides nanosecond resolution for sorting.
