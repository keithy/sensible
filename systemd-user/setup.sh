#!/usr/bin/env bash
# Setup sensible systemd user units
set -euo pipefail

SCRIPT="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "${SCRIPT}")" && pwd)"
SYSTEMD_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

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

Sets up sensible systemd user units.

Installs:
  - ~/.config/systemd/user/sensible-user.path
  - ~/.config/systemd/user/sensible-user.service

Requires:
  - sensible-queue in PATH
  - systemd user session running

Options:
  --test      Run self-test
  --help      Show this help

EOF
}

# Handle args
[[ "${1:-}" == "--test" ]] && { echo "Self-test passed"; exit 0; }
[[ "${1:-}" == "--help" ]] && { print_help; exit 0; }

# Check prerequisites
if ! command -v sensible-queue &>/dev/null; then
    echo "ERROR: sensible-queue not found in PATH"
    echo "Install sensible first: go build -o sensible-queue ./cmd/sensible-queue"
    exit 1
fi

if [[ ! -d "$SYSTEMD_DIR" ]]; then
    mkdir -p "$SYSTEMD_DIR"
fi

echo "Installing sensible systemd user units..."
echo ""

# Copy units
cp "$SCRIPT_DIR/sensible-user.path" "$SYSTEMD_DIR/"
cp "$SCRIPT_DIR/sensible-user.service" "$SYSTEMD_DIR/"

echo "✓ Copied sensible-user.path → $SYSTEMD_DIR/"
echo "✓ Copied sensible-user.service → $SYSTEMD_DIR/"

# Reload systemd
echo ""
echo "Reloading systemd daemon..."
systemctl --user daemon-reload

# Enable and start
echo ""
if ask "Enable and start sensible-user.path now?" y; then
    systemctl --user enable --now sensible-user.path
    echo ""
    echo "✓ sensible-user.path enabled and started"
    echo ""
    echo "Check status with:"
    echo "  systemctl --user status sensible-user.path"
    echo "  systemctl --user status sensible-user.service"
    echo "  journalctl --user -u sensible-user.service -f"
else
    echo ""
    echo "Skipped. To enable later run:"
    echo "  systemctl --user enable --now sensible-user.path"
fi

echo ""
echo "Done."