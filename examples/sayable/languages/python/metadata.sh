#!/usr/bin/env bash
# Update Python package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's/^description = "[^"]*"/description = "'"${PT_DESCRIPTION}"'"/' \
    "${ROOT}/age/impl/python/pyproject.toml"
_sed_i 's/^license = "[^"]*"/license = "'"${PT_LICENSE}"'"/' \
    "${ROOT}/age/impl/python/pyproject.toml"
_sed_i 's/authors = \[{ name = "[^"]*" }\]/authors = [{ name = "'"${PT_AUTHOR}"'" }]/' \
    "${ROOT}/age/impl/python/pyproject.toml"
_sed_i 's|^Repository = "[^"]*"|Repository = "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/age/impl/python/pyproject.toml"
_sed_i 's|^Documentation = "[^"]*"|Documentation = "'"${PT_REPOSITORY}"'/tree/main/age/impl/python#readme"|' \
    "${ROOT}/age/impl/python/pyproject.toml"
_sed_i 's|^Issues = "[^"]*"|Issues = "'"${PT_REPOSITORY}"'/issues"|' \
    "${ROOT}/age/impl/python/pyproject.toml"
