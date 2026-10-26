#!/usr/bin/env bash
# Run one library's native D unit tests with dub.
set -euo pipefail

dub test
