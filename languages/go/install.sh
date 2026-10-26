#!/usr/bin/env bash
# Install Go
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install go; }
install_linux() {
    if ! has go; then
        ARCH="$(dpkg --print-architecture)"
        curl -fsSL "https://go.dev/dl/go1.21.13.linux-${ARCH}.tar.gz" | $SUDO tar -C /usr/local -xz
    fi
}

# golangci-lint must be built with a Go at least as new as the toolchain it
# checks; an older build fails every package with "could not import ...
# export data version". Build it with the local Go when it isn't.
_golangci_lint_current() {
    golangci-lint version 2>/dev/null | grep -q "built with $(go env GOVERSION)"
}
deps() {
    _golangci_lint_current && return
    local existing
    existing="$(command -v golangci-lint 2>/dev/null || true)"
    log "Building golangci-lint with $(go env GOVERSION)..."
    GOBIN="${existing:+$(dirname "${existing}")}" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
}

log "Installing Go..."
"install_${OS}"
deps
