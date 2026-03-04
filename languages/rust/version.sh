#!/usr/bin/env bash
# Update Rust version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/pub const VERSION: &str = "[^"]*"/pub const VERSION: \&str = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/rust/src/formatter.rs"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/rust/Cargo.toml"
if has cargo; then
    (cd "${ROOT}/sayable-age/src/rust" && cargo generate-lockfile)
fi
