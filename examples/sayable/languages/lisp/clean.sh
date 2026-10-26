#!/usr/bin/env bash
# Remove one library's compiled SBCL fasl files.
set -euo pipefail

rm -f ./*.fasl
