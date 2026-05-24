#!/usr/bin/env bash

. "$(dirname "$0")/lib/bash-spec.sh"

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="${ROOT_DIR}/build"

describe "sensible-health (wrapper path resolution)" && {

  SENSIBLE_TASKS_DIR=$(mktemp -d)
  export SENSIBLE_TASKS_DIR
  mkdir -p "${SENSIBLE_TASKS_DIR}/pending" "${SENSIBLE_TASKS_DIR}/done"

  cleanup() {
    rm -rf "$SENSIBLE_TASKS_DIR"
  }
  trap cleanup EXIT

  it "responds via wrapper" && {
    "${BUILD_DIR}/sensible" health | grep -q "healthy"
    should_succeed
  }

  it "can be called directly" && {
    "${BUILD_DIR}/sensible-health" | grep -q "healthy"
    should_succeed
  }

  it "check subcommand outputs pong when healthy" && {
    "${BUILD_DIR}/sensible-health" check | grep -q "pong"
    should_succeed
  }

  it "outputs JSON with status field" && {
    output=$("${BUILD_DIR}/sensible" health)
    echo "$output" | grep -q '"status":"healthy"'
    should_succeed
  }

  it "field selector returns status value" && {
    "${BUILD_DIR}/sensible" health status | grep -q "healthy"
    should_succeed
  }
}
