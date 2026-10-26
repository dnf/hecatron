#!/usr/bin/env bash
# Install Go
source "$(dirname "$0")/../_common.sh"

_go_root="/usr/local/go"

install_macos() { brew_install go; }

# The official tarball, latest stable release (distribution packages lag:
# Ubuntu 24.04 has Go 1.22, too old to build golangci-lint). Unpacked into a
# clean directory, since extracting over an older Go leaves stale files that
# break builds, and linked into /usr/local/bin, which every shell has on PATH.
install_linux() {
    export PATH="${_go_root}/bin:${PATH}"
    has go || _download_go
    local tool
    for tool in go gofmt; do
        $SUDO ln -sfn "${_go_root}/bin/${tool}" "/usr/local/bin/${tool}"
    done
}
_download_go() {
    local version arch
    version="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1)"
    arch="$(dpkg --print-architecture)"
    log "Downloading ${version} for linux-${arch}..."
    $SUDO rm -rf "${_go_root}"
    curl -fsSL "https://go.dev/dl/${version}.linux-${arch}.tar.gz" | $SUDO tar -C "$(dirname "${_go_root}")" -xz
}

# golangci-lint must be built with a Go at least as new as the toolchain it
# checks; an older build fails every package with "could not import ...
# export data version". Build it with the local Go when it isn't.
_golangci_lint_current() {
    golangci-lint version 2>/dev/null | grep -q "built with $(go env GOVERSION)"
}
# Where golangci-lint goes when it isn't installed yet. Off macOS, GOPATH/bin
# isn't on PATH, but Debian's and Ubuntu's default ~/.profile adds ~/.local/bin.
_default_gobin() {
    if [ "${OS}" = "linux" ]; then echo "${HOME}/.local/bin"; else go env GOPATH | sed 's|$|/bin|'; fi
}
deps() {
    local gobin existing
    gobin="$(_default_gobin)"
    export PATH="${gobin}:${PATH}"
    _golangci_lint_current && return
    existing="$(command -v golangci-lint 2>/dev/null || true)"
    [ -n "${existing}" ] && gobin="$(dirname "${existing}")"
    log "Building golangci-lint with $(go env GOVERSION)..."
    GOBIN="${gobin}" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
}

log "Installing Go..."
"install_${OS}"
deps
