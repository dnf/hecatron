# The Node.js package manager a package uses (npm, pnpm, Yarn or Bun), for
# the TypeScript and JavaScript hooks (_hooks.sh) and installers. Sourced,
# not executed. A pnpm or Yarn workspace's package.json names dependencies
# as "workspace:*", which npm can't install.
# shellcheck shell=bash

# Corepack would otherwise ask before downloading the version a project names.
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0
# npm and pnpm would box a newer release's notice into every log.
export npm_config_update_notifier=false

# The package manager's name: the one package.json's "packageManager" field
# names ("pnpm@9.15.0"), else the one whose lockfile is there, else npm. A
# workspace's packages share its root's, so this directory is searched first,
# then each above it, up to the project root (HECATRON_ROOT).
node_package_manager() {
    local _root _dir _declared
    _dir="$(pwd -P)"
    _root="$(cd "${HECATRON_ROOT:-.}" 2>/dev/null && pwd -P)" || _root="${_dir}"
    case "${_dir}/" in "${_root}/"*) ;; *) _root="${_dir}" ;; esac
    while :; do
        if [ -f "${_dir}/package.json" ]; then
            _declared="$(python3 -c '
import json, sys
try:
    print(str(json.load(open(sys.argv[1])).get("packageManager") or "").split("@")[0])
except (ValueError, AttributeError):
    pass
' "${_dir}/package.json")"
            if [ -n "${_declared}" ]; then
                echo "${_declared}"
                return 0
            fi
        fi
        if [ -f "${_dir}/pnpm-lock.yaml" ] || [ -f "${_dir}/pnpm-workspace.yaml" ]; then echo pnpm; return 0; fi
        if [ -f "${_dir}/yarn.lock" ]; then echo yarn; return 0; fi
        if [ -f "${_dir}/bun.lockb" ] || [ -f "${_dir}/bun.lock" ]; then echo bun; return 0; fi
        if [ -f "${_dir}/package-lock.json" ]; then echo npm; return 0; fi
        [ "${_dir}" != "${_root}" ] || break
        _dir="$(dirname "${_dir}")"
    done
    echo npm
}

# The package manager's name, into node_pm_name, and the command that runs
# it, into node_pm: Corepack for pnpm and Yarn, which runs the version
# packageManager names (or its own default) without installing it globally;
# else the tool itself.
node_pm_name=npm
node_pm=(npm)
node_pm_command() {
    node_pm_name="$(node_package_manager)"
    case "${node_pm_name}" in
        pnpm | yarn)
            if command -v corepack >/dev/null 2>&1; then
                node_pm=(corepack "${node_pm_name}")
            else
                node_pm=("${node_pm_name}")
            fi ;;
        *) node_pm=("${node_pm_name}") ;;
    esac
}

# The subcommand that installs the package's dependencies: npm ci with a
# package-lock.json, for an install that matches it.
node_install_command() {
    if [ "${node_pm_name}" = npm ] && [ -f package-lock.json ]; then echo ci; else echo install; fi
}

# For installers (with _common.sh): the package manager this package needs,
# when it isn't there. Corepack runs pnpm and Yarn; Node.js bundles it up to
# version 24, and later versions need it installed. Bun is its own tool.
node_package_manager_ensure() {
    node_pm_command
    case "${node_pm_name}" in
        pnpm | yarn)
            has corepack || has "${node_pm_name}" || _node_global_install corepack corepack ;;
        bun)
            has bun || _node_global_install bun oven-sh/bun/bun ;;
    esac
    node_pm_command
}
# A tool from npm's registry into Node.js's global prefix, linked into
# /usr/local/bin with node and npm (install.sh); Homebrew's formula on macOS.
_node_global_install() {
    local _prefix
    if [ "${OS}" = "macos" ]; then
        brew_install "$2"
        return
    fi
    log "Installing $1 (a component's package manager)..."
    _prefix="$(npm prefix -g)"
    ${SUDO} npm install -g "$1"
    if [ "${_prefix}/bin" != /usr/local/bin ]; then
        ${SUDO} ln -sfn "${_prefix}/bin/$1" "/usr/local/bin/$1"
    fi
}
