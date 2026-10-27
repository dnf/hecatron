# Shared by the Pascal hooks (sourced, not run as a hook): a Makefile's
# targets when the directory has one (native_hook), else Free Pascal on the
# sources directly. Test programs are test*.pas, *_test.pas and the programs
# in tests/ or test/; they find units in the directory and in src/. Every
# compiler output goes to build/, which clean removes.
# shellcheck shell=bash
source "$(dirname "$0")/../../_hooks.sh"

_fpc_build_dir="build"

# The source files in the directory and src/ that aren't tests.
fpc_sources() {
    local _file
    for _file in ./*.pas ./*.pp src/*.pas src/*.pp; do
        [ -f "${_file}" ] || continue
        case "$(basename "${_file}")" in
            test*.pas | *_test.pas) ;;
            *) echo "${_file}" ;;
        esac
    done
}
fpc_tests() {
    local _file
    for _file in ./test*.pas ./*_test.pas tests/*.pas test/*.pas; do
        [ -f "${_file}" ] && echo "${_file}"
    done
    return 0
}

# Compile one source with every output in build/.
fpc_compile() {
    local _search=(-Fu.)
    [ -d src ] && _search+=(-Fusrc)
    mkdir -p "${_fpc_build_dir}"
    run fpc -FE"${_fpc_build_dir}" -FU"${_fpc_build_dir}" "${_search[@]}" "$@"
}

has_pascal_sources() { [ -n "$(fpc_sources)$(fpc_tests)" ]; }
uses_makefile() { [ -f Makefile ] || [ -f makefile ] || [ -f GNUmakefile ]; }
