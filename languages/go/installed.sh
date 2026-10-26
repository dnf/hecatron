#!/usr/bin/env bash
# The Go toolchains installed on this machine, as "version<TAB>home" (home is
# the GOROOT, with bin/go): the go on PATH, /usr/local/go, and the toolchains
# Go itself downloaded (GOTOOLCHAIN) into the module cache. Offline.
source "$(dirname "$0")/../_common.sh"

_goroots() {
    local go dir
    for go in "$(command -v go 2>/dev/null)" /usr/local/go/bin/go; do
        [ -x "${go}" ] && GOTOOLCHAIN=local "${go}" env GOROOT 2>/dev/null
    done
    if has go; then
        for dir in "$(GOTOOLCHAIN=local go env GOMODCACHE 2>/dev/null)"/golang.org/toolchain@*; do
            [ -x "${dir}/bin/go" ] && echo "${dir}"
        done
    fi
    return 0
}

_goroots | sort -u | while IFS= read -r goroot; do
    version="$(sed -n '1s/^go//p' "${goroot}/VERSION" 2>/dev/null)"
    [ -n "${version}" ] && printf '%s\t%s\n' "${version}" "${goroot}"
done
