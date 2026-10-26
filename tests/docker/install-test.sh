#!/usr/bin/env bash
# Check one language's toolchain on a fresh container: tests/install/check.sh
# run as a non-root user with sudo on the given distribution image.
#
#   tests/docker/install-test.sh ubuntu:24.04 go
set -euo pipefail

if [ "$#" -ne 2 ]; then
    echo "usage: $0 <base-image> <lang>" >&2
    exit 2
fi
_base="$1"
_lang="$2"
_here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

exec "${_here}/run.sh" "${_base}" bash /hecatron/tests/install/check.sh "${_lang}"
