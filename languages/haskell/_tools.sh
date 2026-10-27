# Shared by the Haskell installer and doctor (sourced, not executed).
# shellcheck shell=bash

# The tools the components need and lack, one per line: Stack where one has a
# stack.yaml, HLint where one configures it (.hlint.yaml).
haskell_tools_missing() {
    local _dir _stack=0 _hlint=0
    while IFS= read -r _dir; do
        [ -f "${_dir}/stack.yaml" ] && _stack=1
        [ -f "${_dir}/.hlint.yaml" ] && _hlint=1
    done < <(impl_dirs haskell)
    if [ "${_stack}" -eq 1 ] && ! has stack && [ ! -x "${HOME}/.ghcup/bin/stack" ]; then echo stack; fi
    if [ "${_hlint}" -eq 1 ] && ! has hlint; then echo hlint; fi
    return 0
}
