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
    "${BUILD_DIR}/sensible" info | jq -e '.status == "OK"' > /dev/null
    should_succeed
  }

  it "can be called directly" && {
    echo $SENSIBLE_TASKS_DIR
    "${BUILD_DIR}/sensible-info" | jq -e '.status == "OK"' > /dev/null
    should_succeed
  }

  it "outputs JSON with status field" && {
    echo $SENSIBLE_TASKS_DIR
    output=$("${BUILD_DIR}/sensible" info)
    echo "$output" | jq -e '.status == "OK"' > /dev/null
    should_succeed
  }

  it "field selector returns status value" && {
    echo $SENSIBLE_TASKS_DIR
    result=$("${BUILD_DIR}/sensible" info status)
    [ "$result" = "OK" ]
    should_succeed
  }

  it "path selector returns nested value" && {
    echo $SENSIBLE_TASKS_DIR
    result=$("${BUILD_DIR}/sensible" info config.port)
    [ "$result" = "2222" ]
    should_succeed
  }
}
