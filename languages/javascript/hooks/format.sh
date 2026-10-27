#!/usr/bin/env bash
# Default format hook: the package's Prettier or Biome (see format_node in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_node
