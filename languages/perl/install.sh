#!/usr/bin/env bash
# Install Perl (system perl on macOS, apt on Linux), with the build tools the
# components need (see perl_tools_missing in _common.sh) and the CPAN
# modules they depend on
source "$(dirname "$0")/../_common.sh"

install_macos() { log "Using system Perl (no install needed)."; }
install_linux() { apt_install perl; }

deps() {
    local _tool
    while IFS= read -r _tool; do
        case "${_tool}" in
            make) if [ "${OS}" = "linux" ]; then apt_install make; else xcode-select --install || true; fi ;;
            Module::Install) if [ "${OS}" = "linux" ]; then apt_install libmodule-install-perl; else _cpan Module::Install; fi ;;
            Module::Build) if [ "${OS}" = "linux" ]; then apt_install libmodule-build-perl; else _cpan Module::Build; fi ;;
            Perl::Critic) if [ "${OS}" = "linux" ]; then apt_install libperl-critic-perl; else _cpan Perl::Critic; fi ;;
            *) _cpan "${_tool}" ;;
        esac
    done < <(perl_tools_missing)
    each_impl perl _cpan_deps
}
# A component's CPAN dependencies, test requirements included (its cpanfile,
# or what its Makefile.PL or Build.PL declares), through cpanminus,
# system-wide like the tools above; XS modules need a C compiler.
_cpan_deps() {
    [ -f cpanfile ] || [ -f Makefile.PL ] || [ -f Build.PL ] || return 0
    _cpanm
    cpanm --notest --installdeps ${SUDO:+--sudo} .
}
_cpanm() {
    has cpanm && return 0
    if [ "${OS}" = "linux" ]; then apt_install cpanminus build-essential; else brew_install cpanminus; fi
}
# A module from CPAN through cpanminus, into the system Perl's root-owned
# site directory (macOS's has no packages for these).
_cpan() {
    _cpanm
    cpanm --notest ${SUDO:+--sudo} "$1"
}

log "Installing Perl..."
"install_${OS}"
deps
