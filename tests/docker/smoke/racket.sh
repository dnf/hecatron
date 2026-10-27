# Smoke test for racket: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf '#lang racket/base\n(require rackunit)\n(check-equal? (* 6 7) 42)\n(displayln (* 6 7))\n' > hello.rkt
raco make hello.rkt
[ "$(racket hello.rkt)" = "42" ]
raco test --help >/dev/null
