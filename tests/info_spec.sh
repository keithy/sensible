#!/usr/bin/env bash

. "$(dirname "$0")/lib/bash-spec.sh"

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="${ROOT_DIR}/build"

setUp() {
  SENSIBLE_TASKS_DIR=$(mktemp -d)
  export SENSIBLE_TASKS_DIR
  mkdir -p "${SENSIBLE_TASKS_DIR}/pending" "${SENSIBLE_TASKS_DIR}/done"
}

tearDown() {
  rm -rf "$SENSIBLE_TASKS_DIR"
}

trap "tearDown ; output_results"  EXIT

describe "sensible-info (wrapper path resolution)" && {

  setUp

  it "responds via wrapper" && {
    echo $SENSIBLE_TASKS_DIR
    output=$("${BUILD_DIR}/sensible" info)
    expect "$output" to_match '"status":"OK"'
  }

  it "can be called directly" && {
    echo $SENSIBLE_TASKS_DIR
    output=$("${BUILD_DIR}/sensible-info")
    expect "$output" to_match '"status":"OK"'
  }

  it "outputs JSON with status field" && {
    echo $SENSIBLE_TASKS_DIR
    output=$("${BUILD_DIR}/sensible" info)
    expect "$output" to_match '"status":"OK"'
  }

  it "field selector returns status value" && {
    echo $SENSIBLE_TASKS_DIR
    result=$("${BUILD_DIR}/sensible" info status)
    expect "$result" to_be "OK"
  }

  it "path selector returns nested value" && {
    echo $SENSIBLE_TASKS_DIR
    result=$("${BUILD_DIR}/sensible" info port)
    expect "$result" to_be "2222"
  }

  it "config returns file contents" && {
    echo $SENSIBLE_TASKS_DIR
    result=$("${BUILD_DIR}/sensible" info config)
    expect "$result" to_be ""
  }
}
