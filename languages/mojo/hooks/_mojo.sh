# Shared by the Mojo hooks (sourced, not run as a hook). Mojo has no build or
# test runner of its own: the packages are the directories with an
# __init__.mojo in src/ (else in the directory itself), the tests are the
# test_*.mojo and *_test.mojo files anywhere in the directory, each run
# as a program (a testing.TestSuite main), and build output goes to build/.
# shellcheck shell=bash
source "$(dirname "$0")/../../_hooks.sh"

_mojo_build_dir="build"

# Where the packages live: src/ when there is one.
mojo_source_root() { if [ -d src ]; then echo src; else echo .; fi; }

mojo_packages() {
    local _init
    for _init in "$(mojo_source_root)"/*/__init__.mojo; do
        [ -f "${_init}" ] && dirname "${_init}"
    done
    return 0
}
mojo_tests() {
    find . -path "./${_mojo_build_dir}" -prune -o -path './.*' -prune -o \
        -type f \( -name 'test_*.mojo' -o -name '*_test.mojo' \) -print | sort
}
mojo_files() {
    find . -path "./${_mojo_build_dir}" -prune -o -path './.*' -prune -o \
        -type f \( -name '*.mojo' -o -name '*.🔥' \) -print | sort
}
