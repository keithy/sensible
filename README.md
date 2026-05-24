# Sensible

**AI agent → host execution via execlineb scripts**

Container signals host to execute actions via shared filesystem, using execlineb for safety.

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│ Convenience Layer (optional)                                 │
│   sensible <cmd>     # Wrapper delegating to subcommands    │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ CLI Layer (disk queue)                                      │
│   sensible-do         # Enqueue scripts                    │
│   sensible-consume     # Process queue                       │
│   sensible-status      # Check results                      │
│   sensible-list        # List pending                        │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ Systemd Layer (automation)                                  │
│   systemd-path-*      # Watches pending/ via inotify         │
│   systemd-system-*   # Triggers consume on directory change  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│ HTTP/JSON Layer (remote execution)                          │
│   sensible-server     # HTTP API server                     │
│   sensible-client     # HTTP client                         │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Host: start worker (systemd user units)
cd systemd-path-user && ./setup.sh

# Container: enqueue scripts
sensible-do "echo hello" "make build"

# Check result
sensible-status <file_id>
sensible-status <file_id> stdout  # verbatim output
```

## Why execline?

Shell is fundamentally unsafe for AI execution. Execline provides guardrails:

- No `$VAR` interpolation (use `import -env` explicitly)
- No command chaining with `;` or `&&`
- No `-c` option to execute strings

Even with whitelist bypass, execline prevents shell injection.

## Commands

| Command | Description |
|---------|-------------|
| `sensible-do <script> [<script>...]` | Enqueue execlineb script(s), chains implicitly |
| `sensible-consume` | One-shot worker, processes all ready tasks |
| `sensible-status <file_id> [field]` | Check task result (JSON or specific field) |
| `sensible-list` | List pending tasks |

### Chaining

```bash
sensible-do "compile" "test" "deploy"
```

Creates dependency chain: `compile` → `test` → `deploy`

### Status

```bash
sensible-status <file_id>          # JSON output
sensible-status <file_id> stdout   # verbatim stdout
sensible-status <file_id> status   # just the status value
```

## Configuration

Config file: `/etc/sensible/config.json` or `~/.config/sensible/config.json`

```json
{
  "whitelist": ["^echo", "^make"],
  "blacklist": ["^rm -rf", "^dd"]
}
```

- Regex patterns for fine-grained control
- Whitelist takes precedence over blacklist
- Empty whitelist = all allowed

## Systemd Setup (User Units)

```bash
# User units (recommended)
cd systemd-path-user && ./setup.sh

# Or system units (requires root)
cd systemd-path-system && sudo ./setup.sh
```

The path unit watches `pending/` directory. New file triggers `sensible consume` via the wrapper.

## Remote Access

Two options for remote execution:

### SSH

Lock down SSH to only allow sensible commands via forced command:

```bash
# In ~/.ssh/authorized_keys on host:
command="/usr/local/bin/sensible" ssh-rsa AAAA...
```

```bash
ssh host do "echo hello"     # → sensible do "echo hello"
ssh host status <file_id>    # → sensible status <file_id>
ssh host list                # → sensible list
```

Lock to specific subcommand:
```bash
command="/usr/local/bin/sensible do" ssh-rsa AAAA...
```

### HTTP/JSON

```bash
# Host: start server
sensible-server

# Container: use client
sensible-client do "echo hello"
sensible-client status <file_id>
```

Server supports API key authentication via `Authorization: Bearer <key>`.

## Directory Structure

```
${SENSIBLE_TASKS_DIR}/
├── pending/                    # Tasks waiting to execute
│   └── 2026-04-30T12:00:00.123456789Z-script-1.json
└── done/                       # Completed tasks
    └── 2026-04-30T12:00:05.987654321Z-script-1.json
```

## Installation

```bash
# User installation (no sudo)
make install-user

# System installation (requires sudo)
sudo make install-system
```

User install creates:
```
~/.local/bin/sensible              # wrapper
~/.local/lib/sensible/             # subcommands
~/.local/lib/sensible/plugins/     # user plugins
~/.config/sensible/config.json     # config
```

System install creates:
```
/usr/local/bin/sensible
/usr/local/lib/sensible/
/usr/local/lib/sensible/plugins/
/etc/sensible/config.json
```

## Building

```bash
make build          # Build all binaries to build/
make test           # Run tests (Go + bash-spec)
```

## Project Structure

```
sensible/
├── cmd/
│   ├── sensible-do/        # Enqueue scripts
│   ├── sensible-consume/   # Worker
│   ├── sensible-status/    # Check result
│   └── sensible-list/      # List pending
├── pkg/sensible/          # Library
├── systemd-path-user/      # User systemd units
├── systemd-path-system/    # System systemd units
└── tests/                  # bash-spec 2.1 tests
```

## Testing

```bash
make test           # Go tests + bash-spec tests
bash tests/config_spec.sh  # Just bash tests
```

## License

MIT