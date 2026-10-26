#!/usr/bin/env bash
# Remove one library's Free Pascal build outputs.
set -euo pipefail

rm -rf build ./*.o ./*.ppu
