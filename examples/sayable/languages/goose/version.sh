#!/usr/bin/env bash
# Update Goose version in source and the test that checks it
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/let VERSION = "[^"]*"/let VERSION = "'"${ver}"'"/' \
    "${ROOT}/age/impl/goose/sayableage/sayableage.goose"
_sed_i 's/expect("version", sayableage::VERSION, true, "[^"]*")/expect("version", sayableage::VERSION, true, "'"${ver}"'")/' \
    "${ROOT}/age/impl/goose/tests/test_format_age.goose"
