#!/usr/bin/env bash
# Default docs hook: Codox, through lein-codox or a deps.edn :codox alias, into target/doc
# (see docs_clojure in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_clojure
