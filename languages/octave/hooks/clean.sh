#!/usr/bin/env bash
# Default clean hook: Octave's .m files are interpreted, so leave nothing to
# clean.
source "$(dirname "$0")/../../_hooks.sh"
skip "Octave .m files leave no build artifacts"
