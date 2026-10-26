#!/usr/bin/env bash
# Analyze one Dart library ($1: age, date, duration or time).
set -euo pipefail
dart pub get >/dev/null
dart analyze --fatal-infos
