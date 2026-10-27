#!/usr/bin/env bash
# Default format hook: Fourmolu or Ormolu, where configured (see format_haskell in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_haskell
