#!/usr/bin/env bash
# Default build hook: Octave's .m files are interpreted.
source "$(dirname "$0")/../../_hooks.sh"
skip "Octave .m files need no build"
