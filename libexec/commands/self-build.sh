#!/usr/bin/env bash
# hecatron self-build: build the Go binary from this checkout, for working on
# hecatron itself. bin/hecatron then runs it on any branch (a release's
# binary only at its own commit), so hecatron's own tests and lint go through
# hct as they would for users. Run it again after changing Go code: a binary
# built earlier keeps running until then. HECATRON_NO_GO=1 keeps to the
# scripts.
set -euo pipefail

_home="${HECATRON_HOME:?}"
_binary="${_home}/bin/.hecatron-go"
command -v go >/dev/null || { echo "hecatron: self-build needs Go (go.dev/dl, or hct install go)" >&2; exit 1; }
(cd "${_home}" && go build -trimpath -o "${_binary}.new" ./cmd/hecatron)
mv "${_binary}.new" "${_binary}"
# "dev" instead of a commit: bin/hecatron runs it whatever is checked out.
echo dev > "${_binary}.commit"
echo "Built ${_binary} from $(cd "${_home}" && git describe --always --dirty 2>/dev/null || echo "this checkout")"
