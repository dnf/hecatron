#!/usr/bin/env bash
# Install Go
source "$(dirname "$0")/../_common.sh"

_go_root="/usr/local/go"

install_macos() { brew_install go; }

# The official tarball of the default version (distribution packages lag:
# Ubuntu 24.04 has Go 1.22, too old to build golangci-lint). Unpacked into a
# clean directory, since extracting over an older Go leaves stale files that
# break builds, and linked into /usr/local/bin, which every shell has on PATH.
install_linux() {
    export PATH="${_go_root}/bin:${PATH}"
    has go || _download_go "$(default_version)"
    local tool
    for tool in go gofmt; do
        $SUDO ln -sfn "${_go_root}/bin/${tool}" "/usr/local/bin/${tool}"
    done
}
_download_go() {
    local arch
    arch="$(dpkg --print-architecture)"
    log "Downloading Go $1 for linux-${arch}..."
    $SUDO rm -rf "${_go_root}"
    curl -fsSL "https://go.dev/dl/go$1.linux-${arch}.tar.gz" | $SUDO tar -C "$(dirname "${_go_root}")" -xz
}

# Every other version the project's components need, side by side: Go
# downloads a toolchain named by GOTOOLCHAIN into its module cache, where
# installed.sh finds it.
_stable_releases() {
    curl -fsSL 'https://go.dev/dl/?mode=json&include=all' |
        python3 -c 'import json, sys; [print(r["version"][2:]) for r in json.load(sys.stdin) if r["stable"]]'
}
_install_versions() {
    local specs spec version
    specs="$(missing_versions go)"
    while IFS= read -r spec; do
        [ -n "${spec}" ] || continue
        version="$(_stable_releases | newest_match "${spec}")"
        if [ -z "${version}" ]; then
            echo "No Go release matches ${spec}" >&2
            return 1
        fi
        log "Downloading Go ${version}..."
        GOTOOLCHAIN="go${version}" go version </dev/null
    done <<<"${specs}"
}

# golangci-lint must be built with a Go at least as new as the toolchain it
# checks; an older build fails every package with "could not import ...
# export data version". Build it with the newest installed Go when it isn't.
_installed() { bash "$(dirname "$0")/installed.sh"; }
_newest_installed() { _installed | cut -f1 | newest_match 1; }
_golangci_lint_current() {
    local built
    built="$(golangci-lint version 2>/dev/null | sed -n 's/.*built with go\([0-9.]*\).*/\1/p')"
    [ -n "${built}" ] && [ "$(printf '%s\n%s\n' "${built}" "$(_newest_installed)" | newest_match 1)" = "${built}" ]
}
# Where golangci-lint goes when it isn't installed yet. Off macOS, GOPATH/bin
# isn't on PATH, but Debian's and Ubuntu's default ~/.profile adds ~/.local/bin.
_default_gobin() {
    if [ "${OS}" = "linux" ]; then echo "${HOME}/.local/bin"; else go env GOPATH | sed 's|$|/bin|'; fi
}
_build_golangci_lint() {
    local gobin existing newest go
    gobin="$(_default_gobin)"
    export PATH="${gobin}:${PATH}"
    _golangci_lint_current && return
    existing="$(command -v golangci-lint 2>/dev/null || true)"
    [ -n "${existing}" ] && gobin="$(dirname "${existing}")"
    newest="$(_newest_installed)"
    go="$(_installed | awk -F'\t' -v version="${newest}" '$1 == version {print $2 "/bin/go"; exit}')"
    log "Building golangci-lint with $(GOTOOLCHAIN=local "${go}" env GOVERSION)..."
    GOBIN="${gobin}" GOTOOLCHAIN=local "${go}" install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
}
deps() {
    _install_versions
    _build_golangci_lint
}

log "Installing Go..."
"install_${OS}"
deps
