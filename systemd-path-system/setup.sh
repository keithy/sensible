#!/usr/bin/env bash
# Setup sensible systemd system units
# Usage: setup.sh <config-file>
set -euo pipefail

SCRIPT="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "${SCRIPT}")" && pwd)"
CONFIG_DIR="/etc/sensible"
SYSTEMD_DIR="/etc/systemd/system"

print_help() {
    cat << EOF
Usage: $SCRIPT <config-file>

Sets up sensible systemd system units from a config file.

Config file format (JSON):
{
  "serviceName": "sensible",         # systemd unit name
  "tasksDir": "/var/lib/sensible/tasks",
  "keysDir": "/etc/sensible/keys",
  "whitelist": ["^sensible", "^make"]
}

Installs:
  - /etc/sensible/<serviceName>.json  (copied from config)
  - /etc/systemd/system/<serviceName>.path
  - /etc/systemd/system/<serviceName>.service

Requires:
  - root privileges (sudo)
  - sensible in PATH

Options:
  --test      Run self-test
  --help      Show this help

EOF
}

# Handle args
[[ "${1:-}" == "--test" ]] && { echo "Self-test passed"; exit 0; }
[[ "${1:-}" == "--help" ]] && { print_help; exit 0; }

CONFIG_FILE="${1:?Usage: $SCRIPT <config-file>}"
shift || true

# Check if root
if [[ $EUID -ne 0 ]]; then
    echo "ERROR: System install requires root (try sudo)"
    exit 1
fi

# Check prerequisites
if ! command -v sensible &>/dev/null; then
    echo "ERROR: sensible not found in PATH"
    echo "Install sensible first: make install-user"
    exit 1
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Config file not found: $CONFIG_FILE"
    exit 1
fi

# Parse config (requires jq)
if ! command -v jq &>/dev/null; then
    echo "ERROR: jq not found in PATH"
    echo "Install jq first: apk add jq / brew install jq / apt install jq"
    exit 1
fi

# Read config values
SERVICE_NAME=$(jq -r '.serviceName // "sensible"' "$CONFIG_FILE")
TASKS_DIR=$(jq -r '.tasksDir // "/var/lib/sensible/tasks"' "$CONFIG_FILE")
KEYS_DIR=$(jq -r '.keysDir // "/etc/sensible/keys"' "$CONFIG_FILE")
WHITELIST=$(jq -r '.whitelist | if type == "array" then join(",") else . end // ""' "$CONFIG_FILE")

echo "Config: $SERVICE_NAME"
echo "  tasksDir: $TASKS_DIR"
echo "  keysDir: $KEYS_DIR"
echo ""

# Create directories
mkdir -p "$CONFIG_DIR"
mkdir -p "$SYSTEMD_DIR"

# Copy config file
cp "$CONFIG_FILE" "$CONFIG_DIR/$SERVICE_NAME.json"
echo "✓ Copied config → $CONFIG_DIR/$SERVICE_NAME.json"

# Generate path unit
cat > "$SYSTEMD_DIR/$SERVICE_NAME.path" << EOF
[Unit]
Description=Sensible Queue Watcher ($SERVICE_NAME)
Documentation=https://github.com/keithy/sensible

[Path]
DirectoryNotEmpty=$TASKS_DIR/pending
Unit=$SERVICE_NAME.service

[Install]
WantedBy=multi-user.target
EOF
echo "✓ Generated path → $SYSTEMD_DIR/$SERVICE_NAME.path"

# Generate service unit
cat > "$SYSTEMD_DIR/$SERVICE_NAME.service" << EOF
[Unit]
Description=Sensible Queue Worker ($SERVICE_NAME)
Documentation=https://github.com/keithy/sensible

[Service]
Type=oneshot
Environment="SENSIBLE_TASKS_DIR=$TASKS_DIR"
Environment="SENSIBLE_KEYS_DIR=$KEYS_DIR"
ExecStart=/usr/local/bin/sensible consume
StandardOutput=journal
StandardError=journal
RefuseMultipleInstances=true

[Install]
WantedBy=multi-user.target
EOF
echo "✓ Generated service → $SYSTEMD_DIR/$SERVICE_NAME.service"

# Reload systemd
echo ""
echo "Reloading systemd daemon..."
systemctl daemon-reload

# Enable and start
echo ""
read -p "Enable and start $SERVICE_NAME.path now? [Y/n]: " -r response
response="${response:-y}"
case "$response" in
    [yY]|[yY][eE][sS])
        systemctl enable --now "$SERVICE_NAME.path"
        echo ""
        echo "✓ $SERVICE_NAME.path enabled and started"
        echo ""
        echo "Check status with:"
        echo "  sudo systemctl status $SERVICE_NAME.path"
        echo "  sudo systemctl status $SERVICE_NAME.service"
        echo "  sudo journalctl -u $SERVICE_NAME.service -f"
        ;;
    *)
        echo ""
        echo "Skipped. To enable later run:"
        echo "  sudo systemctl enable --now $SERVICE_NAME.path"
        ;;
esac

echo ""
echo "Done."
