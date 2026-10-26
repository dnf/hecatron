#!/usr/bin/env bash
# Remove one library's Raku precompilation cache.
set -euo pipefail

rm -rf .precomp lib/.precomp
