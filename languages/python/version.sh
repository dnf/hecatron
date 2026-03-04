#!/usr/bin/env bash
# Update Python version in source and manifests
source "$(dirname "$0")/../_common.sh"

ver="${1:?version required}"
_sed_i 's/__version__ = "[^"]*"/__version__ = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/python/src/sayable_age/format_age.py"
_sed_i 's/^version = "[^"]*"/version = "'"${ver}"'"/' \
    "${ROOT}/sayable-age/src/python/pyproject.toml"
if has uv; then
    (cd "${ROOT}/sayable-age/src/python" && uv lock)
fi
