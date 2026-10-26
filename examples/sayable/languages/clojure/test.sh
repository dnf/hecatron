#!/usr/bin/env bash
# Run one library's native Clojure test suite.
set -euo pipefail

clojure -M:test
