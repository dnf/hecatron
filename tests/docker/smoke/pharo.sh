# Smoke test for pharo: evaluate an expression in a Pharo 12 image.
# shellcheck shell=bash
set -euo pipefail
curl -fsSL https://get.pharo.org/64/120 | bash >/dev/null
result="$(timeout 120 pharo Pharo.image eval '3 + 4')"
echo "3 + 4 = ${result}"
[ "${result}" = "7" ]
