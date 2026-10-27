#!/usr/bin/env bash
# Default deps hook: pub get, held to pubspec.lock (see deps_dart in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_dart
