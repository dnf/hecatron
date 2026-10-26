#!/usr/bin/env bash
# Build and run one library's Haskell (cabal) test suite.
set -euo pipefail

cabal build
cabal test --test-show-details=direct
