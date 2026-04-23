# Sensible

Secure-by-default remote task execution for Groan CLIs.

Sensible is HTTP/JSON native task daemon with execline execution. Bootstrap via SSH + makeself, operate via HTTP.

## Architecture

### Two-Phase Trust Model

**Phase 1: Bootstrap (SSH, privileged)**
```
Admin → ssh root@host → makeself installer → sensible installed + api-key + whitelist
```
- One-time setup by privileged user
- Creates trust anchor
- Generates API keys
- Configures allowed actions

**Phase 2: Runtime (HTTP/JSON)**
```
AI → HTTP POST {"action": "compile"} → sensible daemon → JSON response
```
- No more SSH needed after install
- API key auth
- execline hardened execution

### How it Works

```
┌─────────────┐     HTTP/JSON      ┌──────────────────┐     execline      ┌─────────┐
│  AI/Groan  │ ──────────────────► │   sensibled       │ ─────────────────► │ actions │
│    CLI     │ ◄────────────────── │   (daemon)       │ ◄───────────────── │  dir    │
└─────────────┘    JSON response    └──────────────────┘    stdout/stderr    └─────────┘
     │                                       │
     │           API key auth                │
     └─────────────────────────────────────┘
```

## Security Model

### execline Execution

Using execline (not shell) provides inherent protection:
- **No shell interpolation** — variables use `import -env`, not `$VAR`
- **No command chaining** — `&&` and `;` are not shell operators
- **No shell escape** — `execlineb "$file"` not `-c "$(cat)"`, prevents injection
- **Builtin-only control flow** — `if`, `try`, `background` builtins

### Layered Validation

```
HTTP Request
    ↓
API Key (Bearer token)
    ↓
JSON Schema validation
    ↓
Action whitelist
    ↓
Args validation (optional)
    ↓
execline execution
    ↓
JSON response
```

Even if API key is compromised, whitelist restricts actions. Even if whitelist bypassed, execline prevents shell injection.

## Installation

### Build Installer

```bash
# Build sensible binary
go build -o sensible ./cmd/sensible

# Create makeself installer
makeself.sh sensible sensible-installer.sh "Sensible" "./sensible install"
```

### Bootstrap Host

```bash
# Single host
sensible deploy --host web1 --ssh-user root --ssh-key ~/.ssh/id_ed25519 --installer sensible-installer.sh

# Multiple hosts
sensible deploy --hosts hosts.txt --ssh-user root --ssh-key ~/.ssh/id_ed25519 --installer sensible-installer.sh
```

The deploy command:
1. SCP installer to host
2. SSH and run installer with `--install`
3. Start sensible daemon via systemd
4. Return endpoint URL and API key

### Runtime

```bash
# Execute action
sensible run web1 compile --target=linux

# Check status
sensible status --host web1

# List actions
sensible list --host web1
```

## HTTP API

### POST /v1/tasks

Submit a task for execution.

**Request:**
```bash
curl -X POST https://web1:8443/v1/tasks \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{"action": "compile", "args": ["--target=linux"], "timeout": 300}'
```

**Response:**
```json
{
  "id": "task-1234",
  "request": {"action": "compile", "args": ["--target=linux"]},
  "status": "success",
  "exit_code": 0,
  "stdout": "Build complete\n",
  "stderr": "",
  "duration_ms": 45230,
  "timestamp": "2026-04-23T17:00:00Z"
}
```

### GET /v1/tasks/:id

Get task result.

### GET /v1/actions

List allowed actions.

### GET /v1/health

Health check (no auth required).

## Configuration

### Whitelist Config (`/etc/sensible/whitelist.yaml`)

```yaml
actions:
  - name: compile
    args_schema:
      target: "^(linux|darwin|windows)$"
    timeout: 600
  - name: restart
    args_schema: {}
    timeout: 60
  - name: update
    args_schema: {}
    timeout: 300
  - name: test
    args_schema: {}
    timeout: 300
```

### API Keys (`/etc/sensible/keys/`)

```
/etc/sensible/keys/
├── default.pem      # Default key for clients
├── admin.pem        # Key with admin privileges
└── ai-client.pem    # Key for AI agents
```

Generate new key:
```bash
sensible keygen --name ai-client
```

## Project Structure

```
sensible/
├── cmd/
│   └── sensible/
│       ├── main.go          # CLI entry point
│       └── install.go       # Install subcommand
├── pkg/
│   ├── daemon/              # HTTP server + task execution
│   │   ├── server.go
│   │   ├── executor.go      # execline execution
│   │   ├── validator.go     # whitelist validation
│   │   └── handler.go      # HTTP handlers
│   ├── deploy/             # SSH + makeself deployment
│   │   └── deploy.go
│   └── config/
│       └── config.go
├── actions/                 # Built-in actions (execline scripts)
├── Makefile
└── README.md
```

## Relationship to Groan

**Groan** = CLI builder (shell scripts → hierarchical CLI)

**Sensible** = Remote execution addon for Groan

```
groan compile --target=linux     # Local execution
sensible run web1 compile        # Remote execution via Groan CLI
```

Sensible executes Groan CLI remotely via execline. Later, sensible will be merged into Groan as the remote execution engine.

## Relationship to host-actions

**host-actions** = Sensible for containers (file queue + dispatch)

**Sensible** = Sensible for hosts (HTTP + SSH bootstrap)

Both share:
- execline execution
- whitelist hardening
- JSON request/response
- CLI-native for AI

Transport layer differs:
- host-actions: volume mount + systemd
- sensible: HTTP + SSH bootstrap

## License

MIT
