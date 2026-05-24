#!/usr/bin/env bash

. "$(dirname "$0")/lib/bash-spec.sh"

describe "IsAllowed with regex patterns" && {

  SENSIBLE_TASKS_DIR=$(mktemp -d)
  export SENSIBLE_TASKS_DIR

  SENSIBLE_DO="$(cd "$(dirname "$0")/.." && pwd)/build/sensible-do"

  run_do() {
    "$SENSIBLE_DO" "$@" 2>/dev/null
  }

  context "with empty whitelist (all allowed)" && {
    it "allows any script" && {
      cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": []}
EOF
      SENSIBLE_CONFIG=/tmp/test-config.json run_do "echo hello"
      should_succeed
    }

    it "allows rm -rf" && {
      cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": []}
EOF
      SENSIBLE_CONFIG=/tmp/test-config.json run_do "rm -rf /"
      should_succeed
    }
  }

  context "with whitelist regex" && {
    it "allows matching regex" && {
      cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["^echo"], "blacklist": []}
EOF
      SENSIBLE_CONFIG=/tmp/test-config.json run_do "echo hello"
      should_succeed
    }

    it "denies non-matching script" && {
      cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["^echo"], "blacklist": []}
EOF
      SENSIBLE_CONFIG=/tmp/test-config.json run_do "make build" 2>/dev/null
      [[ $? -ne 0 ]]
      should_succeed
    }
  }

  context "with blacklist regex" && {
    it "denies matching blacklist regex" && {
      cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": ["^rm"]}
EOF
      SENSIBLE_CONFIG=/tmp/test-config.json run_do "rm -rf /" 2>/dev/null
      [[ $? -ne 0 ]]
      should_succeed
    }

    it "allows non-blacklisted script" && {
      cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": ["^rm"]}
EOF
      SENSIBLE_CONFIG=/tmp/test-config.json run_do "echo hello"
      should_succeed
    }
  }

  context "whitelist overrides blacklist" && {
    it "allows whitelisted despite blacklist match" && {
      cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["^sudo systemctl status"], "blacklist": ["^sudo"]}
EOF
      SENSIBLE_CONFIG=/tmp/test-config.json run_do "sudo systemctl status"
      should_succeed
    }

    it "denies blacklisted not in whitelist" && {
      cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["^sudo systemctl status"], "blacklist": ["^sudo"]}
EOF
      SENSIBLE_CONFIG=/tmp/test-config.json run_do "sudo systemctl restart" 2>/dev/null
      [[ $? -ne 0 ]]
      should_succeed
    }
  }

  # Cleanup
  rm -rf "$SENSIBLE_TASKS_DIR"
}
