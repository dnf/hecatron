#!/usr/bin/env bash
# Install Elixir: the default version and every version the project's
# components need, side by side, from the precompiled releases on
# builds.hex.pm (distributions' Elixir lags: Ubuntu 24.04 has 1.14, which
# current libraries warn about). Each release is built for a few OTP majors
# (1.20 for 27 to 29), so each version runs on an OTP it supports: the
# Erlang on PATH when it is one, else one installed beside it (builds.hex.pm's
# on Ubuntu, Homebrew's erlang@N on macOS). <home>/erlang links to that OTP,
# and hecatron puts its bin/ on PATH with Elixir's.
source "$(dirname "$0")/../_common.sh"

# One directory per version. macOS's is per-user and needs no sudo, and its
# OTPs are Homebrew's.
if [ "${OS}" = "macos" ]; then
    _elixir_root="${HOME}/.local/share/hecatron/elixir"
    _sudo=""
else
    _elixir_root="/opt/elixir"
    _sudo="${SUDO}"
fi
_erlang_root="/opt/erlang"
_builds="https://builds.hex.pm/builds"
# Marks the commands install_linux puts in /usr/local/bin, so uninstall.sh
# knows its own.
_wrapper_marker="hecatron's default Elixir"

# Erlang through its own installer first: Homebrew's newest erlang and
# rebar3, or, on Linux, the distribution's with erlang-dev's headers and
# includes, without which a dependency with a lexer or parser (erlex's .xrl,
# through leex) can't compile.
install_macos() {
    bash "$(dirname "$0")/../erlang/install.sh"
    _install_default
}
install_linux() {
    bash "$(dirname "$0")/../erlang/install.sh"
    _install_default
    # The default version on every shell's PATH, when there's no Elixir at
    # all: wrappers, since each runs on its own OTP.
    if ! has elixir; then
        local tool home
        home="${_elixir_root}/$(default_version)"
        for tool in elixir elixirc iex mix; do
            printf '#!/bin/sh\n# %s (%s), on the OTP it was installed for.\nPATH="%s/bin:%s/erlang/bin:${PATH}" MIX_ARCHIVES="%s/archives"\nexport PATH MIX_ARCHIVES\nexec %s/bin/%s "$@"\n' \
                "${_wrapper_marker}" "$(default_version)" "${home}" "${home}" "${home}" "${home}" "${tool}" |
                $SUDO tee "/usr/local/bin/${tool}" >/dev/null
            $SUDO chmod 0755 "/usr/local/bin/${tool}"
        done
    fi
}

# builds.hex.pm's Elixir releases, one "version<TAB>otp<TAB>sha256" line per
# build (1.20.4 is built for OTP 27, 28 and 29), final releases only.
_elixir_builds=""
_load_elixir_builds() {
    [ -n "${_elixir_builds}" ] && return
    _elixir_builds="$(curl -fsSL "${_builds}/elixir/builds.txt" |
        awk '$1 ~ /^v[0-9]+\.[0-9]+\.[0-9]+-otp-[0-9]+$/ {
            name = substr($1, 2); split(name, part, "-otp-"); print part[1] "\t" part[2] "\t" $4 }')"
}
# The OTP majors an Elixir release is built for, newest first.
_otp_majors() { awk -F'\t' -v version="$1" '$1 == version {print $2}' <<<"${_elixir_builds}" | sort -rn; }

# "major<TAB>root" of an erl: its OTP release and the directory holding it.
# Not on stdin, which erl reads, and the loops calling this are reading.
_otp_of() {
    "$1" -noshell -eval 'io:format("~s\t~s~n", [erlang:system_info(otp_release), code:root_dir()]), halt().' </dev/null 2>/dev/null
}
# The erls an Elixir could run on: the one on PATH first, then those
# installed beside it, newest first.
_erl_candidates() {
    command -v erl 2>/dev/null || true
    local erl
    if [ "${OS}" = "macos" ]; then
        for erl in "$(brew --prefix)"/opt/erlang@*/bin/erl; do
            [ -x "${erl}" ] && echo "${erl}"
        done | sort -rV
    else
        for erl in "${_erlang_root}"/*/bin/erl; do
            [ -x "${erl}" ] && echo "${erl}"
        done | sort -rV
    fi
}
# The root of an installed OTP one of the majors names, or nothing.
_find_otp() {
    local majors="$1" erl major root
    while IFS= read -r erl; do
        [ -n "${erl}" ] || continue
        IFS=$'\t' read -r major root < <(_otp_of "${erl}") || continue
        if grep -qx "${major}" <<<"${majors}"; then
            echo "${root}"
            return
        fi
    done < <(_erl_candidates)
}

# On Ubuntu, builds.hex.pm's OTP build (the ones erlef/setup-beam installs),
# into /opt/erlang/<version>; prints its root, so what it says goes to stderr. Debian has no builds there, so
# a Debian machine runs the Elixirs its own Erlang supports.
_install_otp_linux() {
    local major="$1" arch builds version sha256 work
    case "$(uname -m)" in
        x86_64) arch=amd64 ;;
        aarch64 | arm64) arch=arm64 ;;
        *) return 1 ;;
    esac
    # shellcheck disable=SC1091 # the running system's, not this repository's
    . /etc/os-release
    [ "${ID}" = "ubuntu" ] || return 1
    builds="${_builds}/otp/${arch}/ubuntu-${VERSION_ID}"
    read -r version sha256 < <(curl -fsSL "${builds}/builds.txt" 2>/dev/null |
        awk -v major="${major}" '$1 ~ ("^OTP-" major "(\\.[0-9]+)*$") {print substr($1, 5), $4}' | sort -rV | head -1) || return 1
    [ -n "${version}" ] || return 1
    if [ ! -x "${_erlang_root}/${version}/bin/erl" ]; then
        log "Installing OTP ${version} from builds.hex.pm..." >&2
        work="$(mktemp -d)"
        # Called in a command substitution, where set -e doesn't apply.
        curl -fsSL -o "${work}/otp.tar.gz" "${builds}/OTP-${version}.tar.gz" || return 1
        if [ "$(sha256_of "${work}/otp.tar.gz")" != "${sha256}" ]; then
            rm -rf "${work}"
            echo "error: the OTP ${version} download doesn't match its checksum" >&2
            return 1
        fi
        $SUDO mkdir -p "${_erlang_root}/${version}" &&
            $SUDO tar -C "${_erlang_root}/${version}" --strip-components 1 -xzf "${work}/otp.tar.gz" &&
            (cd "${_erlang_root}/${version}" && $SUDO ./Install -minimal "${_erlang_root}/${version}" >/dev/null) || {
            rm -rf "${work}"
            $SUDO rm -rf "${_erlang_root:?}/${version}"
            return 1
        }
        rm -rf "${work}"
    fi
    echo "${_erlang_root}/${version}"
}
# On macOS, Homebrew's erlang@<major>; prints its root, so what it says goes
# to stderr.
_install_otp_macos() {
    brew info --formula "erlang@$1" >/dev/null 2>&1 || return 1
    brew_install "erlang@$1" >&2
    [ -x "$(brew --prefix "erlang@$1")/bin/erl" ] || return 1
    echo "$(brew --prefix "erlang@$1")/lib/erlang"
}
# An OTP for an Elixir release: one installed that it supports, else the
# newest major it supports that can be installed.
_otp_for() {
    local version="$1" majors major root
    majors="$(_otp_majors "${version}")"
    root="$(_find_otp "${majors}")"
    if [ -z "${root}" ]; then
        while IFS= read -r major; do
            root="$("_install_otp_${OS}" "${major}")" && break
            root=""
        done <<<"${majors}"
    fi
    if [ -z "${root}" ]; then
        local have
        have="$(_otp_of "$(command -v erl 2>/dev/null || echo erl)" | cut -f1)"
        echo "error: Elixir ${version} runs on OTP $(sort -n <<<"${majors}" | paste -sd, -), this machine's Erlang is OTP ${have:-none}, and none of those can be installed here" >&2
        return 1
    fi
    echo "${root}"
}

_install_elixir() {
    local version home root major sha256 work
    _load_elixir_builds
    version="$(cut -f1 <<<"${_elixir_builds}" | sort -u | newest_match "$1")"
    if [ -z "${version}" ]; then
        echo "No Elixir release on builds.hex.pm matches ${1#=}" >&2
        return 1
    fi
    home="${_elixir_root}/${version}"
    # Installed, on an OTP it still supports: an upgrade (Homebrew's erlang,
    # a distribution release) can leave the link at a newer or missing one.
    if [ -x "${home}/bin/elixir" ] && [ -x "${home}/erlang/bin/erl" ] &&
        grep -qx "$(_otp_of "${home}/erlang/bin/erl" | cut -f1)" <<<"$(_otp_majors "${version}")"; then
        return
    fi
    root="$(_otp_for "${version}")" || return 1
    major="$(_otp_of "${root}/bin/erl" | cut -f1)" || true
    if [ -z "${major}" ]; then
        echo "error: the OTP in ${root} doesn't start" >&2
        return 1
    fi
    sha256="$(awk -F'\t' -v version="${version}" -v otp="${major}" '$1 == version && $2 == otp {print $3}' <<<"${_elixir_builds}")"
    log "Installing Elixir ${version} for OTP ${major} from builds.hex.pm..."
    work="$(mktemp -d)"
    curl -fsSL -o "${work}/elixir.zip" "${_builds}/elixir/v${version}-otp-${major}.zip"
    if [ "$(sha256_of "${work}/elixir.zip")" != "${sha256}" ]; then
        rm -rf "${work}"
        echo "error: the Elixir ${version} download doesn't match its checksum" >&2
        return 1
    fi
    python3 -m zipfile -e "${work}/elixir.zip" "${work}/elixir"
    chmod 0755 "${work}"/elixir/bin/*
    ${_sudo} mkdir -p "${_elixir_root}"
    ${_sudo} rm -rf "${home}"
    ${_sudo} mv "${work}/elixir" "${home}"
    ${_sudo} ln -sfn "${root}" "${home}/erlang"
    rm -rf "${work}"
}

# Hex, which mix needs to fetch dependencies, in each version's own archive
# directory (<home>/archives, MIX_ARCHIVES in config.json): an archive built
# on one OTP doesn't load on an older one, so versions can't share ~/.mix's.
# A fresh Elixir has none, and mix asks interactively.
_install_hex() {
    local home="$1" mix_home
    [ -d "${home}/archives" ] && [ -n "$(ls "${home}/archives")" ] && return
    mix_home="$(mktemp -d)"
    ${_sudo} env PATH="${home}/bin:${home}/erlang/bin:${PATH}" MIX_ARCHIVES="${home}/archives" MIX_HOME="${mix_home}" \
        "${home}/bin/mix" local.hex --force --if-missing </dev/null
    ${_sudo} rm -rf "${mix_home}"
}
_install_hex_everywhere() {
    local home
    for home in "${_elixir_root}"/*/; do
        [ -x "${home}bin/mix" ] && [ -x "${home}erlang/bin/erl" ] && _install_hex "${home%/}"
    done
    return 0
}
# Outside a project (or when the project's components all use another
# version), the default still gets installed.
_install_default() { _install_elixir "=$(default_version)"; }

_install_versions() {
    local specs spec
    specs="$(missing_versions elixir)"
    while IFS= read -r spec; do
        [ -n "${spec}" ] || continue
        _install_elixir "${spec}"
    done <<<"${specs}"
}

# rebar3, which mix needs to build Erlang dependencies, for each component's
# own Elixir (mix keeps one per Elixir and OTP under ~/.mix); mix asks
# interactively without it.
_rebar() {
    [ -f mix.exs ] || return 0
    mix local.rebar --force --if-missing </dev/null
}
deps() {
    _install_versions
    _install_hex_everywhere
    impl_has elixir -name mix.exs || return 0
    each_impl_versioned elixir _rebar
}

log "Installing Elixir..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install
