#!/bin/sh
#
# sensible-client - POSIX shell HTTP client for Sensible
# Works with: bash, dash, ash (busybox), zsh
#
# Usage: sensible-client <command> [args]
#

# Config
SENSIBLE_HOST="${SENSIBLE_HOST:-localhost:2222}"
SENSIBLE_URL="http://${SENSIBLE_HOST}/sensible"
AUTH_HEADER="${SENSIBLE_AUTH_HEADER:-}"

usage() {
    cat <<EOF
Usage: sensible-client <command>

Commands:
  sensible-client do <action> [args...]   Execute action via HTTP API
  sensible-client list                    List pending tasks
  sensible-client status <file_id>        Check task status

Environment:
  SENSIBLE_HOST          API host:port (default: localhost:2222)
  SENSIBLE_AUTH_HEADER   Authorization header value

Examples:
  sensible-client do status
  sensible-client do compile --target=linux
  sensible-client list
  sensible-client status 2026-04-30T12:00:00.123Z-compile
EOF
}

# URL encode string (simple version)
url_encode() {
    echo "$1" | sed 's/ /%20/g'
}

# Make HTTP request
http_request() {
    method="$1"
    path="$2"
    
    if [ -n "$AUTH_HEADER" ]; then
        auth_hdr="-H Authorization: Bearer ${AUTH_HEADER}"
    else
        auth_hdr=""
    fi
    
    # Use curl if available
    if command -v curl >/dev/null 2>&1; then
        curl -s -X "$method" $auth_hdr "${SENSIBLE_URL}${path}"
        return $?
    fi
    
    # Fallback to wget (less common but possible)
    if command -v wget >/dev/null 2>&1; then
        wget -q -O - --method="$method" $auth_hdr "${SENSIBLE_URL}${path}"
        return $?
    fi
    
    echo "Error: neither curl nor wget found" >&2
    return 1
}

# Execute action
cmd_do() {
    request="$*"
    
    if [ -z "$request" ]; then
        echo "Error: No action specified" >&2
        exit 1
    fi
    
    echo "Executing: $request"
    
    # Encode request for URL
    encoded_request="$(url_encode "$request")"
    
    # Check HTTP status code
    if command -v curl >/dev/null 2>&1; then
        http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
            -H "Authorization: Bearer ${AUTH_HEADER}" \
            "${SENSIBLE_URL}/sensible?request=${encoded_request}")
    else
        http_code="200"  # Assume success if no curl
    fi
    
    response="$(http_request "POST" "/sensible?request=${encoded_request}")"
    
    if [ "$http_code" = "202" ]; then
        echo "Async task queued"
        file_id=$(echo "$response" | grep '"file_id"' | sed 's/.*":"\(.*\)".*/\1/' | head -1)
        echo "FileID: $file_id"
        echo ""
        echo "Poll with: sensible-client status $file_id"
    else
        echo "$response"
    fi
}

# List pending tasks
cmd_list() {
    response="$(http_request "GET" "/sensible?request=list-pending")"
    
    if [ -z "$response" ] || [ "$response" = "null" ]; then
        echo "No pending tasks"
    else
        echo "$response"
    fi
}

# Check task status
cmd_status() {
    file_id="$1"
    
    if [ -z "$file_id" ]; then
        echo "Error: No file_id specified" >&2
        exit 1
    fi
    
    response="$(http_request "GET" "/sensible/${file_id}")"
    echo "$response"
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