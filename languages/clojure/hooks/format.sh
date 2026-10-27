#!/usr/bin/env bash
# Default format hook: cljfmt, where configured (see format_clojure in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_clojure
