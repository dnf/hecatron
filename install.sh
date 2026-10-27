#!/bin/sh
# Install hecatron, or update it to the latest release, and put it on PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/dnf/hecatron/main/install.sh | sh
#   . ~/.hecatron/install.sh       # from a clone: also sets up this shell
#
# Installs into ~/.hecatron (HECATRON_INSTALL_DIR), at the latest release
# (HECATRON_VERSION=v0.2.1 or main for another). Writes ~/.hecatron/env, and
# adds a line sourcing it to the shell startup files, as rustup and uv do:
# ~/.profile always; ~/.bashrc, ~/.bash_profile, ~/.bash_login, ~/.zshrc and
# ~/.zshenv where they exist; and fish's conf.d. Run again to update; it
# never adds a line twice. At a release, it also downloads that release's Go
# binary for this machine, which bin/hecatron then runs (checksum-verified;
# HECATRON_NO_GO=1 at run time keeps to the scripts). HECATRON_NO_MODIFY_PATH=1 leaves startup files
# alone.
#
# Run from a clone (sourced or not), it uses that clone where it is.
#
# First it installs what hecatron itself needs when it's missing: on macOS the
# Command Line Tools (git, python3), and on Linux git, Python 3 and perl from
# the distribution (apt, dnf or zypper), with sudo when not root.
#
# POSIX sh, and safe to source into bash or zsh: no `exit`, no `set -e`;
# everything lives in functions that are removed at the end.

_hecatron_say() { printf 'hecatron: %s\n' "$*"; }
_hecatron_fail() { printf 'hecatron: error: %s\n' "$*" >&2; return 1; }

_hecatron_sudo() { if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo "$@"; fi; }
_hecatron_python_ok() { python3 -c 'import sys; sys.exit(sys.version_info < (3, 9))' 2>/dev/null; }

# macOS: the Command Line Tools, which hold git and python3 (until they're
# installed, /usr/bin/git and /usr/bin/python3 only open a dialog). Installed
# without the dialog through softwareupdate, as Homebrew's installer does.
_hecatron_macos_clt() {
    xcode-select -p >/dev/null 2>&1 && return 0
    _hecatron_say "Installing the Command Line Tools (git, python3); this needs your password..."
    _hecatron_flag="/tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress"
    touch "${_hecatron_flag}"
    _hecatron_label="$(softwareupdate -l 2>/dev/null |
        sed -n 's/^[* ]*Label: \(Command Line Tools for Xcode.*\)$/\1/p' | sort -V | tail -1)"
    if [ -n "${_hecatron_label}" ]; then
        _hecatron_sudo softwareupdate -i "${_hecatron_label}" --verbose
    fi
    rm -f "${_hecatron_flag}"
    xcode-select -p >/dev/null 2>&1 && return 0
    # softwareupdate didn't offer them: the dialog is the remaining way.
    xcode-select --install >/dev/null 2>&1 || true
    _hecatron_fail "finish installing the Command Line Tools in the window that opened, then run this again"
}

# Linux: git, Python 3.9+ and perl (the parallel runner's timing), from the
# distribution's package manager.
_hecatron_linux_packages() {
    _hecatron_missing=""
    command -v git >/dev/null 2>&1 || _hecatron_missing="${_hecatron_missing} git"
    _hecatron_python_ok || _hecatron_missing="${_hecatron_missing} python3"
    command -v perl >/dev/null 2>&1 || _hecatron_missing="${_hecatron_missing} perl"
    command -v curl >/dev/null 2>&1 || _hecatron_missing="${_hecatron_missing} curl"
    [ -n "${_hecatron_missing}" ] || return 0
    _hecatron_say "Installing what hecatron needs:${_hecatron_missing}..."
    # shellcheck disable=SC2086 # one word per package
    if command -v apt-get >/dev/null 2>&1; then
        _hecatron_sudo apt-get -o Acquire::Retries=3 update -qq &&
            _hecatron_sudo apt-get -o Acquire::Retries=3 install -y -qq ${_hecatron_missing} ca-certificates
    elif command -v dnf >/dev/null 2>&1; then
        _hecatron_sudo dnf install -y -q ${_hecatron_missing}
    elif command -v zypper >/dev/null 2>&1; then
        _hecatron_sudo zypper --non-interactive install ${_hecatron_missing}
    else
        _hecatron_fail "install${_hecatron_missing} with your package manager, then run this again"
        return 1
    fi
}

_hecatron_prerequisites() {
    case "$(uname -s)" in
        Darwin) _hecatron_macos_clt || return 1 ;;
        Linux) _hecatron_linux_packages || return 1 ;;
    esac
    _hecatron_python_ok || {
        _hecatron_fail "hecatron needs Python 3.9 or later; python3 is $(python3 -V 2>&1 || echo missing)"
        return 1
    }
}

# The directory this script is in, when that can be known: when executed, or
# sourced from bash or zsh. Empty when piped (curl | sh).
_hecatron_script_dir() {
    _hecatron_src=""
    if [ -n "${BASH_SOURCE:-}" ]; then
        # shellcheck disable=SC3028,SC3054 # bash only, guarded
        _hecatron_src="${BASH_SOURCE[0]}"
    elif [ -n "${ZSH_VERSION:-}" ]; then
        # zsh only; eval keeps its syntax from other shells' parsers.
        eval '_hecatron_src="${(%):-%x}"'
    else
        case "$0" in
            */install.sh | install.sh) _hecatron_src="$0" ;;
        esac
    fi
    if [ -n "${_hecatron_src}" ] && [ -f "${_hecatron_src}" ]; then
        (cd "$(dirname "${_hecatron_src}")" && pwd)
    fi
}

# A network command, tried up to three times: a transient failure shouldn't
# end the install.
_hecatron_retry() {
    _hecatron_try=1
    until "$@"; do
        [ "${_hecatron_try}" -ge 3 ] && return 1
        _hecatron_say "That failed; trying again in $((_hecatron_try * 5)) s..." >&2
        sleep $((_hecatron_try * 5))
        _hecatron_try=$((_hecatron_try + 1))
    done
}

# Clone, or bring an existing install to the requested version.
_hecatron_fetch() {
    _hecatron_dir="$1"
    _hecatron_repo="${HECATRON_REPO:-https://github.com/dnf/hecatron}"
    _hecatron_version="${HECATRON_VERSION:-}"
    if [ -z "${_hecatron_version}" ]; then
        _hecatron_version="$(_hecatron_retry git ls-remote --tags --refs "${_hecatron_repo}" 'v*' 2>/dev/null |
            sed 's|.*refs/tags/||' | sort -t. -k1,1 -k2,2n -k3,3n | tail -1)"
        [ -n "${_hecatron_version}" ] || _hecatron_version="main"
    fi
    if [ -d "${_hecatron_dir}/.git" ]; then
        _hecatron_say "Updating ${_hecatron_dir} to ${_hecatron_version}..."
        _hecatron_retry git -C "${_hecatron_dir}" fetch --quiet --tags --force "${_hecatron_repo}" "${_hecatron_version}" &&
            git -C "${_hecatron_dir}" checkout --quiet --force FETCH_HEAD
    elif [ -e "${_hecatron_dir}" ]; then
        _hecatron_fail "${_hecatron_dir} exists and isn't a hecatron clone; move it or set HECATRON_INSTALL_DIR"
    else
        _hecatron_say "Installing hecatron ${_hecatron_version} into ${_hecatron_dir}..."
        _hecatron_retry git clone --quiet --branch "${_hecatron_version}" "${_hecatron_repo}" "${_hecatron_dir}"
    fi
}

# The Go binary for this machine (docs/design.md §19) from the release the
# install is at, checksum-verified, with the commit it belongs to; bin/hecatron
# runs it at that commit only. Not at a release (a branch, a clone), an
# unsupported platform, or a failed download: none, and bin/hecatron runs as
# before.
_hecatron_binary() {
    _hecatron_go="$1/bin/.hecatron-go"
    rm -f "${_hecatron_go}" "${_hecatron_go}.commit"
    _hecatron_tag="$(git -C "$1" describe --tags --exact-match 2>/dev/null)" || return 0
    case "$(uname -s)" in
        Darwin) _hecatron_os=darwin ;;
        Linux) _hecatron_os=linux ;;
        *) return 0 ;;
    esac
    case "$(uname -m)" in
        x86_64 | amd64) _hecatron_arch=amd64 ;;
        arm64 | aarch64) _hecatron_arch=arm64 ;;
        *) return 0 ;;
    esac
    _hecatron_asset="hecatron-${_hecatron_os}-${_hecatron_arch}"
    _hecatron_url="${HECATRON_REPO:-https://github.com/dnf/hecatron}/releases/download/${_hecatron_tag}"
    _hecatron_tmp="$(mktemp -d)" || return 0
    if _hecatron_retry curl -fsSL -o "${_hecatron_tmp}/${_hecatron_asset}" "${_hecatron_url}/${_hecatron_asset}" &&
        _hecatron_retry curl -fsSL -o "${_hecatron_tmp}/SHA256SUMS" "${_hecatron_url}/SHA256SUMS"; then
        _hecatron_want="$(awk -v f="${_hecatron_asset}" '$2 == f {print $1}' "${_hecatron_tmp}/SHA256SUMS")"
        _hecatron_got="$( (sha256sum "${_hecatron_tmp}/${_hecatron_asset}" 2>/dev/null ||
            shasum -a 256 "${_hecatron_tmp}/${_hecatron_asset}") | cut -d' ' -f1)"
        if [ -n "${_hecatron_want}" ] && [ "${_hecatron_want}" = "${_hecatron_got}" ]; then
            chmod +x "${_hecatron_tmp}/${_hecatron_asset}" &&
                mv "${_hecatron_tmp}/${_hecatron_asset}" "${_hecatron_go}" &&
                git -C "$1" rev-parse HEAD > "${_hecatron_go}.commit" &&
                _hecatron_say "Installed the ${_hecatron_os}-${_hecatron_arch} binary for ${_hecatron_tag}"
        else
            _hecatron_say "The ${_hecatron_tag} binary's checksum didn't match; using the scripts instead" >&2
        fi
    else
        _hecatron_say "No ${_hecatron_tag} binary for ${_hecatron_os}-${_hecatron_arch}; using the scripts instead" >&2
    fi
    rm -rf "${_hecatron_tmp}"
}

# ~/.hecatron/env (sh, bash, zsh) and env.fish: put bin/ on PATH once.
_hecatron_write_env() {
    _hecatron_bin="$1/bin"
    cat > "$1/env" <<EOF
# hecatron: put the command on PATH (written by install.sh; sourced from your
# shell's startup files).
case ":\${PATH}:" in
    *":${_hecatron_bin}:"*) ;;
    *) export PATH="${_hecatron_bin}:\${PATH}" ;;
esac
EOF
    cat > "$1/env.fish" <<EOF
# hecatron: put the command on PATH (written by install.sh).
if not contains "${_hecatron_bin}" \$PATH
    set -gx PATH "${_hecatron_bin}" \$PATH
end
EOF
}

# Add `. "<dir>/env"  # hecatron` to a startup file, once, replacing a line an
# earlier install wrote for another directory.
_hecatron_add_line() {
    _hecatron_file="$1"
    _hecatron_line="$2"
    if [ -f "${_hecatron_file}" ] && grep -qxF "${_hecatron_line}" "${_hecatron_file}"; then
        return 0
    fi
    if [ -f "${_hecatron_file}" ] && grep -q '  # hecatron$' "${_hecatron_file}"; then
        grep -v '  # hecatron$' "${_hecatron_file}" > "${_hecatron_file}.hecatron-tmp" &&
            cat "${_hecatron_file}.hecatron-tmp" > "${_hecatron_file}" &&
            rm -f "${_hecatron_file}.hecatron-tmp"
    fi
    printf '\n%s\n' "${_hecatron_line}" >> "${_hecatron_file}" &&
        _hecatron_say "Added hecatron to ${_hecatron_file}"
}

_hecatron_modify_path() {
    _hecatron_env_line=". \"$1/env\"  # hecatron"
    _hecatron_add_line "${HOME}/.profile" "${_hecatron_env_line}"
    for _hecatron_rc in .bashrc .bash_profile .bash_login .zshrc .zshenv; do
        [ -f "${HOME}/${_hecatron_rc}" ] && _hecatron_add_line "${HOME}/${_hecatron_rc}" "${_hecatron_env_line}"
    done
    # A zsh user with no startup file yet: zsh doesn't read ~/.profile.
    case "${SHELL:-}" in
        */zsh) [ -f "${HOME}/.zshrc" ] || [ -f "${HOME}/.zshenv" ] ||
            _hecatron_add_line "${HOME}/.zshrc" "${_hecatron_env_line}" ;;
    esac
    if command -v fish >/dev/null 2>&1 || [ -d "${HOME}/.config/fish" ]; then
        mkdir -p "${HOME}/.config/fish/conf.d" &&
            printf 'source "%s/env.fish"  # hecatron\n' "$1" > "${HOME}/.config/fish/conf.d/hecatron.fish"
    fi
}

_hecatron_install() {
    _hecatron_prerequisites || return 1
    _hecatron_here="$(_hecatron_script_dir)"
    if [ -z "${HECATRON_REPO:-}" ] && [ -n "${_hecatron_here}" ] && [ -x "${_hecatron_here}/bin/hecatron" ]; then
        _hecatron_target="${_hecatron_here}"
        _hecatron_say "Using the clone at ${_hecatron_target}"
    else
        _hecatron_target="${HECATRON_INSTALL_DIR:-${HOME}/.hecatron}"
        _hecatron_fetch "${_hecatron_target}" || return 1
    fi
    [ -x "${_hecatron_target}/bin/hecatron" ] || {
        _hecatron_fail "${_hecatron_target} has no bin/hecatron"
        return 1
    }
    _hecatron_binary "${_hecatron_target}"
    _hecatron_write_env "${_hecatron_target}" || return 1
    if [ "${HECATRON_NO_MODIFY_PATH:-0}" != 1 ]; then
        _hecatron_modify_path "${_hecatron_target}" || return 1
    fi
    # Sourced: this shell gets it now. Run as a program: it can't, so say how.
    . "${_hecatron_target}/env"
    _hecatron_say "Installed $("${_hecatron_target}/bin/hecatron" --version)."
    if [ "${_hecatron_sourced}" = 1 ]; then
        _hecatron_say "Ready: try 'hct help'."
    else
        _hecatron_say "New shells have it. To use it in this one, run:"
        printf '\n    . "%s/env"\n\n' "${_hecatron_target}"
    fi
}

# Whether this file was sourced rather than run, where the shell can tell.
_hecatron_sourced=0
if [ -n "${ZSH_VERSION:-}" ]; then
    case "${ZSH_EVAL_CONTEXT:-}" in *:file*) _hecatron_sourced=1 ;; esac
elif [ -n "${BASH_VERSION:-}" ]; then
    # shellcheck disable=SC3028,SC3054 # bash only, guarded
    [ "${BASH_SOURCE[0]}" != "$0" ] && _hecatron_sourced=1
fi

_hecatron_install
_hecatron_status=$?
unset -f _hecatron_say _hecatron_fail _hecatron_retry _hecatron_sudo _hecatron_python_ok _hecatron_macos_clt \
    _hecatron_linux_packages _hecatron_prerequisites _hecatron_script_dir _hecatron_fetch _hecatron_binary _hecatron_write_env \
    _hecatron_add_line _hecatron_modify_path _hecatron_install
unset _hecatron_src _hecatron_dir _hecatron_repo _hecatron_version _hecatron_bin _hecatron_file \
    _hecatron_line _hecatron_env_line _hecatron_rc _hecatron_here _hecatron_target _hecatron_sourced \
    _hecatron_flag _hecatron_label _hecatron_missing _hecatron_try _hecatron_go _hecatron_tag _hecatron_os \
    _hecatron_arch _hecatron_asset _hecatron_url _hecatron_tmp _hecatron_want _hecatron_got
# Sourced: hand back the status without leaving the user's shell; run: exit with it.
return "${_hecatron_status}" 2>/dev/null || exit "${_hecatron_status}"
