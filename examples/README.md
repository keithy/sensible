# Sensible Configuration Examples

## Profiles

### 01-lockdown.json — Goclaw Self-Improve
Only `podman commit` is allowed. Everything else blocked.
- Use case: Container self-improvement with restricted commands
- Blocks: All commands except podman commit

### 02-open-scripting.json — Developer Machine
Allow most commands but block dangerous system operations.
- Use case: Developer workstation with some restrictions
- Blocks: `dd`, `sudo`, `su`, `reboot`, `shutdown`, `mkfs`, `fdisk`, `exec`
- Allows: Most commands (make, go, git, etc.)

### 03-minimal.json — No External Commands
Block all non-builtin commands. Only execline builtins allowed.
- Use case: Very restricted environment
- Blocks: All external commands
- Allows: All execline builtins (cd, export, etc.)

### 04-whitelist-only.json — Explicit Allowlist
Only specific commands allowed.
- Use case: Strict environments, CI/CD pipelines
- Allows: make, go, git, cd, ls, cat, echo, pwd
- Blocks: Everything else

### 05-block-control-flow.json — Block Control Structures
Block execline control flow but allow regular commands.
- Use case: Allow commands but prevent complex scripting
- Blocks: foreground, background, if, for, loop, etc.
- Allows: All other commands plus safe builtins

### 06-ai-monitoring.json — AI Monitoring Agent
Restricted access for AI agents with safe monitoring commands.
- Use case: AI with limited access to peer machines
- Allows: systemctl status, journalctl, dmesg, git, make, go, etc.
- Blocks: Everything else (full shell still available via separate key)

**Use case:** Give an AI a restricted SSH key for peer-to-peer task execution while allowing safe monitoring commands. Human admin keeps full access via separate unrestricted key.

## Usage

```bash
# Setup with lockdown profile
./systemd-path-user/setup.sh examples/01-lockdown.json
```

For peer-to-peer, sensible runs via SSH commands — no systemd involved.

### Setup

Each user on the peer has their own sensible installation and config:

```bash
# On peer: create user
sudo useradd -m -s /usr/bin/nologin peeruser

# On peer: install sensible for user
sudo -u peeruser make install-user
```

### SSH Key Restriction

In `~/.ssh/authorized_keys` on the peer:

```
command="/usr/local/bin/sensible",no-pty,no-agent-forwarding,no-X11-forwarding ssh-rsa AAAA...
```

This restricts the key to only invoke the `sensible` binary — can't run arbitrary commands.

### Workflow

```bash
# You: Queue work to peer's queue
ssh -i your_key peeruser@peerhost sensible do "make build"

# Peer's sensible validates and queues the script
# Script goes to peer's ~/.local/share/sensible/tasks/pending/

# Peer's consume processes the queue (via SSH)
ssh -i your_key peeruser@peerhost sensible consume
```

### Consume Modes

```bash
# Oneshot - process all and exit
sensible consume

# Daemon - run forever (watching for new tasks)
sensible consume -t 0
sensible consume --start

# Timed - exit after idle timeout
sensible consume -t 5m

# Stop a running consume
sensible consume --stop
```

- **oneshot**: Process all pending tasks, then exit
- **daemon** (`-t 0` or `--start`): Run forever, watch for new tasks, stop file exits
- **timed** (`-t <duration>`): Run until idle for the specified duration
- **--stop**: Create stop file to gracefully stop a running consume

### Key Points

- **No systemd** — work triggered on-demand via SSH
- **Validation** happens at `consume` time, not `do` time
- **Config** is the peer's config (`~/.config/sensible/config.json`)
- **Queue** is per-user on the peer
- **SSH restriction** ensures key can only invoke `sensible`, not arbitrary commands
- **Two layers**: SSH restriction + sensible whitelist/blacklist

### Per-User Independence

Each SSH user on the peer has their own:
- Config (`~/.config/sensible/`)
- Queue (`~/.local/share/sensible/tasks/`)

Users can't interfere with each other's sensible instances.

### AI + Human Admin Pattern

One user account can have multiple SSH keys with different restrictions:

```bash
# In ~/.ssh/authorized_keys:

# AI key: restricted to sensible commands only
command="/usr/local/bin/sensible",no-pty,no-agent-forwarding,no-X11-forwarding ssh-rsa AI_KEY...

# Human key: full shell access (normal SSH)
ssh-rsa HUMAN_KEY...
```

- **AI key**: Can only run `sensible do`, `sensible consume`, etc. Restricted by sensible config (e.g., 06-ai-monitoring.json)
- **Human key**: Full shell access for administration

This allows an AI to perform peer-to-peer task execution and monitoring while the human admin retains full control.

## System Installation

For system systemd, create a dedicated user to run sensible (principle of least privilege):

```bash
# Create sensible user
sudo useradd -r -s /usr/bin/nologin sensible

# Setup directories
sudo mkdir -p /var/lib/sensible/tasks
sudo chown sensible:sensible /var/lib/sensible/tasks
```

Then modify the service file to run as the sensible user:

```ini
[Service]
Type=oneshot
User=sensible
Group=sensible
ExecStart=/usr/local/bin/sensible consume
```

## Fields

| Field | Purpose |
|-------|---------|
| `whitelist` | Allowed commands (regex). Empty = all allowed (unless blacklist) |
| `blacklist` | Denied commands (regex). Empty = none denied |
| `builtinBlacklist` | Denied execline builtins (regex) |

## Logic

1. Check `builtinBlacklist` — blocks execline builtins
2. For executors (foreground, if, etc.) — check content recursively
3. Check `whitelist` — if matches, allowed
4. Check `blacklist` — if matches, denied
5. Default — allowed

## Execline Executors (content checked)

- foreground, background
- if, ifnot, ifelse, ifte, ifthenelse
- forx, forstdin, forbacktickx, loopwhilex
- exec, tryexec, pipeline
- backtick (content after braces)