#!/usr/bin/env bash
# Default prune hook: SwiftPM's build configurations unused for 7d, keeping
# the fetched dependencies; with --caches, Xcode's DerivedData folders whose
# project is gone (prune_swift in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
prune_swift
