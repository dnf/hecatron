#!/usr/bin/env bash
# Install Perl (system perl on macOS, apt on Linux), with the build tools the
# components need: make for Makefile.PL, Module::Build for Build.PL, and
# Perl::Critic where a component configures it (.perlcriticrc)
source "$(dirname "$0")/../_common.sh"

install_macos() { log "Using system Perl (no install needed)."; }
install_linux() { apt_install perl; }

deps() {
    local _tool
    while IFS= read -r _tool; do
        case "${_tool}" in
            make) if [ "${OS}" = "linux" ]; then apt_install make; else xcode-select --install || true; fi ;;
            Module::Build) if [ "${OS}" = "linux" ]; then apt_install libmodule-build-perl; else _cpan Module::Build; fi ;;
            Perl::Critic) if [ "${OS}" = "linux" ]; then apt_install libperl-critic-perl; else _cpan Perl::Critic; fi ;;
        esac
    done < <(perl_tools_missing)
    each_impl perl _cpanfile_deps
}
# A component's CPAN dependencies (its cpanfile, test requirements included)
# through cpanminus, system-wide like the tools above; XS modules need a C
# compiler.
_cpanfile_deps() {
    [ -f cpanfile ] || return 0
    if ! has cpanm; then
        if [ "${OS}" = "linux" ]; then apt_install cpanminus build-essential; else brew_install cpanminus; fi
    fi
    cpanm --notest --installdeps ${SUDO:+--sudo} .
}
# macOS's system Perl takes modules from CPAN through cpanminus, into its
# root-owned site directory.
_cpan() {
    has cpanm || brew_install cpanminus
    cpanm --notest ${SUDO:+--sudo} "$1"
}

log "Installing Perl..."
"install_${OS}"
deps
