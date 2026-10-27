#!/usr/bin/env bash
# Default build hook: the package's source tarball, into this directory.
source "$(dirname "$0")/../../_hooks.sh"
[ -f DESCRIPTION ] || skip "no DESCRIPTION, so no package to build"
run R CMD build .
