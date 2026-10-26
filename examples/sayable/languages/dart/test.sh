#!/usr/bin/env bash
# Fetch packages and run the library's dart test suite.
set -euo pipefail
dart pub get
dart test
