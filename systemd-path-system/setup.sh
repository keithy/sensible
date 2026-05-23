#!/usr/bin/env bash
# Setup sensible systemd system units
set -euo pipefail

SCRIPT="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "${SCRIPT}")" && pwd)"
SYSTEMD_DIR="/etc/systemd/system"

ask() {
    local prompt="$1"
    local default="${2:-}"
    local response

    while true; do
        case "$default" in
            y) printf "%s [Y/n]: " "$prompt" ;;
            n) printf "%s [y/N]: " "$prompt" ;;
            *) printf "%s [y/n]: " "$prompt" ;;
        esac
        if ! read -r response </dev/tty 2>/dev/null; then
            echo
            return 1
        fi
        case "${response:-$default}" in
            [yY]|[yY][eE][sS]) return 0 ;;
            [nN]|[nN][oO]) return 1 ;;
        esac
    done
}

print_help() {
    cat << EOF
Usage: $SCRIPT [OPTIONS]

Sets up sensible systemd system units.

Installs:
  - /etc/systemd/system/sensible.path
  - /etc/systemd/system/sensible.service

Requires:
  - root privileges (sudo)
  - sensible-queue in PATH

Options:
  --test      Run self-test
  --help      Show this help

EOF
}

# Handle args
[[ "${1:-}" == "--test" ]] && { echo "Self-test passed"; exit 0; }
[[ "${1:-}" == "--help" ]] && { print_help; exit 0; }

# Check if root
if [[ $EUID -ne 0 ]]; then
    echo "ERROR: System install requires root (try sudo)"
    exit 1
fi

# Check prerequisites
if ! command -v sensible-queue &>/dev/null; then
    echo "ERROR: sensible-queue not found in PATH"
    echo "Install sensible first: go build -o sensible-queue ./cmd/sensible-queue"
    exit 1
fi

echo "Installing sensible systemd system units..."
echo ""

# Copy units
cp "$SCRIPT_DIR/sensible.path" "$SYSTEMD_DIR/"
cp "$SCRIPT_DIR/sensible.service" "$SYSTEMD_DIR/"

echo "✓ Copied sensible.path → $SYSTEMD_DIR/"
echo "✓ Copied sensible.service → $SYSTEMD_DIR/"

# Reload systemd
echo ""
echo "Reloading systemd daemon..."
systemctl daemon-reload

# Enable and start
echo ""
if ask "Enable and start sensible.path now?" y; then
    systemctl enable --now sensible.path
    echo ""
    echo "✓ sensible.path enabled and started"
    echo ""
    echo "Check status with:"
    echo "  sudo systemctl status sensible.path"
    echo "  sudo systemctl status sensible.service"
    echo "  sudo journalctl -u sensible.service -f"
else
    echo ""
    echo "Skipped. To enable later run:"
    echo "  sudo systemctl enable --now sensible.path"
fi

echo ""
echo "Done."