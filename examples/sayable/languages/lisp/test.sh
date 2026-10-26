#!/usr/bin/env bash
# Load one library's Lisp sources in SBCL and run its tests.
set -euo pipefail

case "$1" in
    age)
        sbcl --noinform --non-interactive \
            --eval '(require :asdf)' \
            --eval '(push (truename ".") asdf:*central-registry*)' \
            --eval '(asdf:load-system "sayable-age")' \
            --load test/test-formatter.lisp \
            --eval '(sayable-age-test:run-tests)'
        ;;
    *)
        sbcl --noinform --non-interactive \
            --load package.lisp --load types.lisp --load formatter.lisp \
            --load load-locales.lisp --load test/test-formatter.lisp \
            --eval "(sayable-${1}-test:run-tests)"
        ;;
esac
