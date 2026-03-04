#!/usr/bin/env bash
# Update Python package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's/^description = "[^"]*"/description = "'"${PT_DESCRIPTION}"'"/' \
    "${ROOT}/src/python/pyproject.toml"
_sed_i 's/^license = "[^"]*"/license = "'"${PT_LICENSE}"'"/' \
    "${ROOT}/src/python/pyproject.toml"
_sed_i 's/authors = \[{ name = "[^"]*" }\]/authors = [{ name = "'"${PT_AUTHOR}"'" }]/' \
    "${ROOT}/src/python/pyproject.toml"
_sed_i 's|^Repository = "[^"]*"|Repository = "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/src/python/pyproject.toml"
_sed_i 's|^Documentation = "[^"]*"|Documentation = "'"${PT_REPOSITORY}"'/tree/main/src/python#readme"|' \
    "${ROOT}/src/python/pyproject.toml"
_sed_i 's|^Issues = "[^"]*"|Issues = "'"${PT_REPOSITORY}"'/issues"|' \
    "${ROOT}/src/python/pyproject.toml"
