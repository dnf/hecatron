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
# never adds a line twice. HECATRON_NO_MODIFY_PATH=1 leaves startup files
# alone.
#
# Run from a clone (sourced or not), it uses that clone where it is.
#
# POSIX sh, and safe to source into bash or zsh: no `exit`, no `set -e`;
# everything lives in functions that are removed at the end.

_hecatron_say() { printf 'hecatron: %s\n' "$*"; }
_hecatron_fail() { printf 'hecatron: error: %s\n' "$*" >&2; return 1; }

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

# Clone, or bring an existing install to the requested version.
_hecatron_fetch() {
    _hecatron_dir="$1"
    _hecatron_repo="${HECATRON_REPO:-https://github.com/dnf/hecatron}"
    command -v git >/dev/null 2>&1 || {
        _hecatron_fail "git is needed (macOS: xcode-select --install; Debian/Ubuntu: sudo apt-get install git)"
        return 1
    }
    _hecatron_version="${HECATRON_VERSION:-}"
    if [ -z "${_hecatron_version}" ]; then
        _hecatron_version="$(git ls-remote --tags --refs "${_hecatron_repo}" 'v*' 2>/dev/null |
            sed 's|.*refs/tags/||' | sort -t. -k1,1 -k2,2n -k3,3n | tail -1)"
        [ -n "${_hecatron_version}" ] || _hecatron_version="main"
    fi
    if [ -d "${_hecatron_dir}/.git" ]; then
        _hecatron_say "Updating ${_hecatron_dir} to ${_hecatron_version}..."
        git -C "${_hecatron_dir}" fetch --quiet --tags --force "${_hecatron_repo}" "${_hecatron_version}" &&
            git -C "${_hecatron_dir}" checkout --quiet --force FETCH_HEAD
    elif [ -e "${_hecatron_dir}" ]; then
        _hecatron_fail "${_hecatron_dir} exists and isn't a hecatron clone; move it or set HECATRON_INSTALL_DIR"
    else
        _hecatron_say "Installing hecatron ${_hecatron_version} into ${_hecatron_dir}..."
        git clone --quiet --branch "${_hecatron_version}" "${_hecatron_repo}" "${_hecatron_dir}" 2>/dev/null ||
            git clone --quiet "${_hecatron_repo}" "${_hecatron_dir}"
    fi
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
unset -f _hecatron_say _hecatron_fail _hecatron_script_dir _hecatron_fetch _hecatron_write_env \
    _hecatron_add_line _hecatron_modify_path _hecatron_install
unset _hecatron_src _hecatron_dir _hecatron_repo _hecatron_version _hecatron_bin _hecatron_file \
    _hecatron_line _hecatron_env_line _hecatron_rc _hecatron_here _hecatron_target _hecatron_sourced
# Sourced: hand back the status without leaving the user's shell; run: exit with it.
return "${_hecatron_status}" 2>/dev/null || exit "${_hecatron_status}"
