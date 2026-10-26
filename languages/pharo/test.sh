#!/usr/bin/env bash
# Run one library's Pharo tests on the image downloaded into age/impl/pharo.
set -euo pipefail

_image="${PWD}/../../../age/impl/pharo/Pharo.image"
if [ ! -f "${_image}" ]; then
    (cd "$(dirname "${_image}")" && curl -fsSL https://get.pharo.org/64/120 | bash)
fi
pharo --headless "${_image}" st run_tests.st
