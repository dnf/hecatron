#!/usr/bin/env bash
# Default deps hook: the Clojure CLI's prepare, or lein deps (see deps_clojure in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_clojure
