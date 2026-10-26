#!/usr/bin/env bash
# Build one library's Ada static library with gprbuild.
set -euo pipefail

gprbuild -P "sayable${1}.gpr" -XBUILD_MODE=build
