#!/usr/bin/env bash
##==================================================================================
## Tests for sensible IsAllowed configuration
##==================================================================================

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/bash-spec.sh"

SENSIBLE_TASKS_DIR=$(mktemp -d)
SENSIBLE_ACTIONS_DIR=$(mktemp -d)
export SENSIBLE_TASKS_DIR
export SENSIBLE_ACTIONS_DIR
unset SENSIBLE_CONFIG

cd /code/sensible
go build -o build/sensible-do ./cmd/sensible-do 2>/dev/null
SENSIBLE_DO="${SCRIPT_DIR}/../build/sensible-do"

cleanup() {
    rm -rf "$SENSIBLE_TASKS_DIR" "$SENSIBLE_ACTIONS_DIR"
}
trap cleanup EXIT

run_do() {
    "$SENSIBLE_DO" "$@" 2>/dev/null
}

describe "IsAllowed whitelist/blacklist"
    
    context "with empty config (all allowed)"
        it "allows echo hello"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": []}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "echo hello"
            should_succeed
        end
        
        it "allows rm -rf when whitelist is empty"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": []}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "rm -rf /"
            should_succeed
        end
    end
    
    context "with whitelist config"
        it "allows whitelisted script"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["echo"], "blacklist": []}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "echo hello"
            should_succeed
        end
        
        it "denies non-whitelisted script"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["echo"], "blacklist": []}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "rm -rf /" 2>/dev/null
            [[ $? -ne 0 ]]
            should_succeed
        end
    end
    
    context "with blacklist config"
        it "denies blacklisted script"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": ["rm -rf"]}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "rm -rf /" 2>/dev/null
            [[ $? -ne 0 ]]
            should_succeed
        end
        
        it "allows non-blacklisted script"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": ["rm -rf"]}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "echo hello"
            should_succeed
        end
    end
    
    context "whitelist overrides blacklist"
        it "allows sudo systemctl status (whitelisted)"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["sudo systemctl status"], "blacklist": ["sudo"]}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "sudo systemctl status"
            should_succeed
        end
        
        it "denies sudo systemctl restart (blacklisted)"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["sudo systemctl status"], "blacklist": ["sudo"]}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "sudo systemctl restart" 2>/dev/null
            [[ $? -ne 0 ]]
            should_succeed
        end
    end
    
    context "with regex patterns"
        it "allows matching regex"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["^echo"], "blacklist": []}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "echo hello"
            should_succeed
        end
        
        it "denies matching blacklist regex"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": [], "blacklist": ["^rm"]}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "rm -rf /" 2>/dev/null
            [[ $? -ne 0 ]]
            should_succeed
        end
        
        it "denies non-matching script when whitelist not empty"
            cat > /tmp/test-config.json << 'EOF'
{"whitelist": ["^echo"], "blacklist": []}
EOF
            SENSIBLE_CONFIG=/tmp/test-config.json run_do "make build" 2>/dev/null
            [[ $? -ne 0 ]]
            should_succeed
        end
    end

end

output_results