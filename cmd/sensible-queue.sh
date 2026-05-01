#!/bin/sh
#
# sensible-queue - POSIX shell CLI for local task queue
# Works with: bash, dash, ash (busybox), zsh
#
# Usage: sensible-queue <command> [args]
#

# Config
TASKS_DIR="${SENSIBLE_TASKS_DIR:-/var/lib/sensible/tasks}"
ACTIONS_DIR="${SENSIBLE_ACTIONS_DIR:-/var/lib/sensible/actions}"
PENDING_DIR="${TASKS_DIR}/pending"
DONE_DIR="${TASKS_DIR}/done"

usage() {
    cat <<EOF
Usage: sensible-queue <command>

Commands:
  sensible-queue do <action> [args...]  Execute action directly
  sensible-queue worker                 Run background worker (continuous)
  sensible-queue list                  List pending tasks
  sensible-queue status <file_id>       Check task status

Environment:
  SENSIBLE_TASKS_DIR   Task storage directory (default: /var/lib/sensible/tasks)
  SENSIBLE_ACTIONS_DIR Action scripts directory (default: /var/lib/sensible/actions)
EOF
}

# Get timestamp (RFC3339Nano-like, varies by system)
timestamp() {
    # Try GNU date first, fallback to simple format
    if date -u +"%Y-%m-%dT%H:%M:%S.%N%Z" >/dev/null 2>&1; then
        date -u +"%Y-%m-%dT%H:%M:%S.%N%Z"
    else
        # Busybox/BSD fallback - milliseconds only
        date -u +"%Y-%m-%dT%H:%M:%S.000Z"
    fi
}

# Generate FileID: <timestamp>-<action>
make_file_id() {
    echo "$(timestamp)-${1}"
}

# Check if action is whitelisted
is_whitelisted() {
    case "$1" in
        status|restart|compile|update|test) return 0 ;;
        *) return 1 ;;
    esac
}

# Get timeout for action (seconds)
get_timeout() {
    case "$1" in
        status) echo "10" ;;
        restart) echo "60" ;;
        compile) echo "600" ;;
        update) echo "300" ;;
        test) echo "300" ;;
        *) echo "15" ;;
    esac
}

# Execute action via execlineb
execute_action() {
    request="$1"
    timeout="$2"
    
    tmpfile="$(mktemp /tmp/sensible-XXXXXX.sh)" || return 1
    echo "$request" > "$tmpfile"
    
    # Run with timeout using sh-compatible approach
    # Note: timeout behavior varies - on busybox it may not work as expected
    (
        if command -v timeout >/dev/null 2>&1; then
            timeout "${timeout}s" execlineb "$tmpfile" > "${tmpfile}.out" 2> "${tmpfile}.err"
        else
            execlineb "$tmpfile" > "${tmpfile}.out" 2> "${tmpfile}.err"
        fi
    ) &
    pid=$!
    
    # Simple wait with timeout
    count=0
    while kill -0 "$pid" 2>/dev/null; do
        sleep 1
        count=$((count + 1))
        if [ "$count" -ge "$timeout" ]; then
            kill "$pid" 2>/dev/null
            wait "$pid" 2>/dev/null
            echo "1||timeout||timed out after ${timeout}s||0"
            rm -f "$tmpfile" "${tmpfile}.out" "${tmpfile}.err"
            return
        fi
    done
    
    wait "$pid"
    exit_code=$?
    
    stdout="$(cat "${tmpfile}.out" 2>/dev/null || echo '')"
    stderr="$(cat "${tmpfile}.err" 2>/dev/null || echo '')"
    
    # Get duration (approximate)
    duration=$((count * 1000))
    
    rm -f "$tmpfile" "${tmpfile}.out" "${tmpfile}.err"
    
    echo "${exit_code}||${stdout}||${stderr}||${duration}"
}

# Save task to pending directory
save_pending() {
    file_id="$1"
    task_json="$2"
    
    mkdir -p "$PENDING_DIR" 2>/dev/null || true
    echo "$task_json" > "${PENDING_DIR}/${file_id}.json"
}

# Load task from pending or done
load_task() {
    file_id="$1"
    
    if [ -f "${PENDING_DIR}/${file_id}.json" ]; then
        cat "${PENDING_DIR}/${file_id}.json"
        return 0
    elif [ -f "${DONE_DIR}/${file_id}.json" ]; then
        cat "${DONE_DIR}/${file_id}.json"
        return 0
    fi
    return 1
}

# Move task to done directory
move_to_done() {
    file_id="$1"
    task_json="$2"
    
    mkdir -p "$DONE_DIR" 2>/dev/null || true
    echo "$task_json" > "${DONE_DIR}/${file_id}.json"
}

# Delete task from pending
delete_pending() {
    rm -f "${PENDING_DIR}/${file_id}.json"
}

# Get file modification time (portable)
get_mtime() {
    file="$1"
    if [ -f "$file" ]; then
        # Try stat -c %Y (GNU), fallback to ls -l format
        if stat -c %Y "$file" >/dev/null 2>&1; then
            stat -c %Y "$file"
        else
            # BSD/macOS fallback
            stat -f %m "$file" 2>/dev/null || echo "0"
        fi
    else
        echo "0"
    fi
}

# List pending tasks
list_pending() {
    if [ ! -d "$PENDING_DIR" ]; then
        echo "No pending tasks"
        return
    fi
    
    count=0
    for f in "$PENDING_DIR"/*.json; do
        [ -f "$f" ] || continue
        count=$((count + 1))
        
        # Extract fields with grep/sed (portable)
        file_id="$(basename "$f" .json)"
        request="$(grep '"request"' "$f" 2>/dev/null | sed 's/.*:"\(.*\)".*/\1/' | head -1)"
        depends_on="$(grep '"depends_on"' "$f" 2>/dev/null | sed 's/.*:"\(.*\)".*/\1/' | head -1)"
        
        dep_msg=""
        if [ -n "$depends_on" ]; then
            dep_msg=" (depends on: $depends_on)"
        fi
        
        echo "  $file_id  $request$dep_msg"
    done
    
    if [ "$count" -eq 0 ]; then
        echo "No pending tasks"
    fi
}

# Command: do
cmd_do() {
    request="$*"
    
    if [ -z "$request" ]; then
        echo "Error: No action specified" >&2
        exit 1
    fi
    
    action="${request%% *}"
    
    if ! is_whitelisted "$action"; then
        echo "Error: Action '$action' not whitelisted" >&2
        exit 1
    fi
    
    timeout="$(get_timeout "$action")"
    
    echo "Executing: $request (timeout: ${timeout}s)"
    
    result="$(execute_action "$request" "$timeout")"
    
    # Parse result
    IFS='||' read -r exit_code stdout stderr duration <<EOF
$result
EOF
    
    if [ "$exit_code" -eq 0 ]; then
        status="success"
    else
        status="failed"
    fi
    
    echo "Status: $status"
    echo "Exit code: $exit_code"
    [ -n "$stdout" ] && echo "Stdout:" && echo "$stdout"
    [ -n "$stderr" ] && echo "Stderr:" && echo "$stderr"
    echo "Duration: ${duration}ms"
}

# Command: list
cmd_list() {
    echo "Pending tasks:"
    list_pending
}

# Command: status
cmd_status() {
    file_id="$1"
    
    if [ -z "$file_id" ]; then
        echo "Error: No file_id specified" >&2
        exit 1
    fi
    
    if ! load_task "$file_id"; then
        echo "Error: Task not found: $file_id" >&2
        exit 1
    fi
}

# Command: worker
cmd_worker() {
    echo "Starting worker, processing pending tasks..."
    
    while true; do
        oldest_mtime=""
        oldest_file=""
        
        # Find oldest pending task
        for f in "$PENDING_DIR"/*.json; do
            [ -f "$f" ] || continue
            
            mtime="$(get_mtime "$f")"
            if [ -z "$oldest_mtime" ] || [ "$mtime" -lt "$oldest_mtime" ]; then
                oldest_mtime=$mtime
                oldest_file=$f
            fi
        done
        
        if [ -z "$oldest_file" ]; then
            sleep 1
            continue
        fi
        
        file_id="$(basename "$oldest_file" .json)"
        
        # Read task JSON
        json="$(cat "$oldest_file")"
        
        # Extract fields
        request="$(echo "$json" | grep '"request"' | sed 's/.*:"\(.*\)".*/\1/' | head -1)"
        depends_on="$(echo "$json" | grep '"depends_on"' | sed 's/.*:"\(.*\)".*/\1/' | head -1)"
        status="$(echo "$json" | grep '"status"' | sed 's/.*:"\(.*\)".*/\1/' | head -1)"
        
        [ -z "$status" ] && status="queued"
        
        # Check dependency
        if [ -n "$depends_on" ]; then
            if parent_json="$(load_task "$depends_on")"; then
                parent_status="$(echo "$parent_json" | grep '"status"' | sed 's/.*:"\(.*\)".*/\1/' | head -1)"
                if [ "$parent_status" != "success" ] && [ "$parent_status" != "failed" ]; then
                    sleep 1
                    continue
                fi
            else
                sleep 1
                continue
            fi
        fi
        
        if [ "$status" != "queued" ]; then
            sleep 1
            continue
        fi
        
        if [ -z "$request" ]; then
            delete_pending "$file_id"
            continue
        fi
        
        echo "Executing: $file_id (request: $request)"
        
        action="${request%% *}"
        timeout="$(get_timeout "$action")"
        
        result="$(execute_action "$request" "$timeout")"
        
        # Parse result
        IFS='||' read -r exit_code stdout stderr duration <<EOF
$result
EOF
        
        if [ "$exit_code" -eq 0 ]; then
            new_status="success"
        else
            new_status="failed"
        fi
        
        # Update task (simple approach - just write new file)
        timestamp_now="$(timestamp)"
        
        move_to_done "$file_id" "{\"id\":\"$action\",\"file_id\":\"$file_id\",\"request\":\"$request\",\"status\":\"$new_status\",\"exit_code\":$exit_code,\"stdout\":\"$stdout\",\"stderr\":\"$stderr\",\"duration_ms\":$duration,\"timestamp\":\"$timestamp_now\"}"
        delete_pending "$file_id"
        
        echo "Completed: $file_id (status: $new_status)"
    done
}

# Main
case "${1:-}" in
    do)
        shift
        cmd_do "$@"
        ;;
    list)
        cmd_list
        ;;
    status)
        shift
        cmd_status "$1"
        ;;
    worker)
        cmd_worker
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        if [ -z "$1" ]; then
            usage
        else
            echo "Unknown command: $1" >&2
            usage
            exit 1
        fi
        ;;
esac