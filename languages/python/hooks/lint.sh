#!/usr/bin/env bash
# Default lint hook: Ruff, with the project's own Ruff configuration if any.
source "$(dirname "$0")/../../_hooks.sh"
run uvx ruff check .
