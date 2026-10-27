#!/usr/bin/env bats

setup() {
    source "${BATS_TEST_DIRNAME}/../add.sh"
}

@test "adds two numbers" {
    run add 2 3
    [ "${status}" -eq 0 ]
    [ "${output}" = "5" ]
}
