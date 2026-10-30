#!/usr/bin/env bash
# Default deps hook: an Xcode app's CocoaPods (see deps_objectivec in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_objectivec
