#!/usr/bin/env bash
# Default clean hook: the Clojure CLI or Leiningen, whichever the directory uses (see clojure_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
clojure_hook clean
