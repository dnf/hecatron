# Shared by the Haskell installer and doctor (sourced, not executed).
# shellcheck shell=bash

# The formatters hecatron format runs, at these versions.
_ORMOLU_VERSION="0.9.0.0"
_FOURMOLU_VERSION="0.21.0.0"

# The tools the components need and lack, one per line: Stack where one has a
# stack.yaml, HLint where one configures it (.hlint.yaml), and Fourmolu or
# Ormolu where one configures it (fourmolu.yaml, .ormolu).
haskell_tools_missing() {
    local _dir _stack=0 _hlint=0
    while IFS= read -r _dir; do
        [ -f "${_dir}/stack.yaml" ] && _stack=1
        [ -f "${_dir}/.hlint.yaml" ] && _hlint=1
    done < <(impl_dirs haskell)
    if [ "${_stack}" -eq 1 ] && ! has stack && [ ! -x "${HOME}/.ghcup/bin/stack" ]; then echo stack; fi
    if [ "${_hlint}" -eq 1 ] && ! has hlint; then echo hlint; fi
    if [ "$(fourmolu --version 2>/dev/null | head -1 | cut -d' ' -f2)" != "${_FOURMOLU_VERSION}" ] &&
        impl_configures haskell fourmolu.yaml; then
        echo fourmolu
    fi
    # macOS gets Homebrew's Ormolu, whichever version that is (install.sh).
    if ! { [ "${OS}" = macos ] && has ormolu; } &&
        [ "$(ormolu --version 2>/dev/null | head -1 | cut -d' ' -f2)" != "${_ORMOLU_VERSION}" ] &&
        impl_configures haskell .ormolu; then
        echo ormolu
    fi
    return 0
}
