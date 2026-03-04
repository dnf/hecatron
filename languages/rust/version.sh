#!/usr/bin/env bash
# Update Rust version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/pub const VERSION: &str = "[^"]*"/pub const VERSION: \&str = "'"${ver}"'"/' \
    "${ROOT}/age/impl/rust/src/formatter.rs"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/age/impl/rust/Cargo.toml"
if has cargo; then
    (cd "${ROOT}/age/impl/rust" && cargo generate-lockfile)
fi
