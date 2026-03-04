#!/usr/bin/env bash
# Update Rust package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's/^description = "[^"]*"/description = "'"${PT_DESCRIPTION}"'"/' \
    "${ROOT}/age/impl/rust/Cargo.toml"
_sed_i 's/^license = "[^"]*"/license = "'"${PT_LICENSE}"'"/' \
    "${ROOT}/age/impl/rust/Cargo.toml"
_sed_i 's/^authors = \["[^"]*"\]/authors = ["'"${PT_AUTHOR}"'"]/' \
    "${ROOT}/age/impl/rust/Cargo.toml"
_sed_i 's|^repository = "[^"]*"|repository = "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/age/impl/rust/Cargo.toml"
_sed_i 's|^homepage = "[^"]*"|homepage = "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/age/impl/rust/Cargo.toml"
