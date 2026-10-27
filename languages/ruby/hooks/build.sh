#!/usr/bin/env bash
# Default build hook: each gemspec's gem, into the directory.
source "$(dirname "$0")/../../_hooks.sh"
exists '*.gemspec' || skip "no gemspec to build"
for _spec in ./*.gemspec; do
    run gem build "${_spec}"
done
