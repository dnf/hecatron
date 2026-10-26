#!/usr/bin/env bash
# Check that golangci-lint was built with the installed Go (see install.sh).
source "$(dirname "$0")/../_common.sh"

if has golangci-lint && ! golangci-lint version 2>/dev/null | grep -qF "built with $(go env GOVERSION)"; then
    problem "golangci-lint was built with a different Go than $(go env GOVERSION)"
fi
doctor_done
