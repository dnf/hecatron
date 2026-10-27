# Shared by the Ada hooks (sourced, not run as a hook): Alire when the
# directory is a crate (alire.toml), else GPRbuild on its project files. The
# library or program is each top-level .gpr that isn't a test project; the test
# projects are tests/*.gpr, test/*.gpr and a top-level *test*.gpr, and their
# mains are the tests.
# shellcheck shell=bash
source "$(dirname "$0")/../../_hooks.sh"

is_alire_crate() { [ -f alire.toml ]; }
alr_() { run alr --non-interactive "$@"; }

gpr_projects() {
    local _file
    for _file in ./*.gpr; do
        [ -f "${_file}" ] || continue
        case "$(basename "${_file}")" in
            *test*) ;;
            *) echo "${_file}" ;;
        esac
    done
}
gpr_test_projects() {
    local _file
    for _file in ./*test*.gpr tests/*.gpr test/*.gpr; do
        [ -f "${_file}" ] && echo "${_file}"
    done
    return 0
}
# The executables a built project's mains link to: gprclean -n lists the
# files it would delete, executables among them.
gpr_mains() {
    local _file
    while IFS= read -r _file; do
        [ -f "${_file}" ] && [ -x "${_file}" ] || continue
        case "${_file}" in
            *.so | *.so.* | *.dylib | *.dll) ;;
            *) echo "${_file}" ;;
        esac
    done < <(gprclean -n -P "$1")
}
