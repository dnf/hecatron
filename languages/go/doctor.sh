#!/usr/bin/env bash
# Check that golangci-lint was built with a Go at least as new as every
# installed one (see install.sh).
source "$(dirname "$0")/../_common.sh"

if has golangci-lint; then
    built="$(golangci-lint version 2>/dev/null | sed -n 's/.*built with go\([0-9.]*\).*/\1/p')"
    newest="$(bash "$(dirname "$0")/installed.sh" | cut -f1 | newest_match 1)"
    if [ -n "${newest}" ] && [ "$(printf '%s\n%s\n' "${built:-0}" "${newest}" | newest_match 1)" != "${built}" ]; then
        problem "golangci-lint was built with Go ${built:-unknown}, older than the installed Go ${newest}"
    fi
fi
doctor_done
