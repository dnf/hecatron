#!/usr/bin/env bash
# Default docs hook: rustdoc through cargo doc --no-deps, into the target directory's doc/
# (see docs_rust in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_rust
