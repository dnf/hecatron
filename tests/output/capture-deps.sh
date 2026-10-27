#!/usr/bin/env bash
# Captures the dependency tools' reports hecatron audit and hecatron
# outdated read (internal/deps, libexec/lib/audit.py and outdated.py), for
# the formats tests/audit/reports and tests/outdated/reports don't have
# yet: pnpm, Yarn 1, Yarn 2+ and Bun, and OSV-Scanner on every audit
# fixture's lock file. Runs the commands languages/_audit.sh and
# _outdated.sh run, on tests/audit/fixtures and tests/deps/fixtures, and
# writes tests/output/<tool>/<version>/{audit,outdated}-<format>-<case>.*
# under CAPTURE_OUT (default: tests/output). Needs the network; installs
# Node.js, the package managers, uv and OSV-Scanner, so run it in a
# throwaway container:
#
#   docker run --rm -v "$PWD:/hecatron:ro" -v "$PWD/tests/output:/out" \
#       -e CAPTURE_OUT=/out hecatron-test:ubuntu-24.04 bash /hecatron/tests/output/capture-deps.sh
set -euo pipefail

_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_out="${CAPTURE_OUT:-${_home}/tests/output}"
_OSV_SCANNER_VERSION="2.7.0"
_OSV_SCANNER_SHA256="160c6253e2922e9e0a6a9771c5c4c40a4b3c6090b34a3cdd5e80c96cc2e4218b"
_work="$(mktemp -d)"
export PATH="${_work}/bin:${_work}/node/bin:${HOME}/.local/bin:${PATH}"
export npm_config_update_notifier=false

# Node.js's current LTS line, from its own index.
install_node() {
    local _version _file
    _version="$(curl -fsSL https://nodejs.org/dist/index.json |
        python3 -c 'import json,sys; print(next(r["version"] for r in json.load(sys.stdin) if r["lts"]))')"
    _file="node-${_version}-linux-x64.tar.xz"
    curl -fsSL -o "${_work}/${_file}" "https://nodejs.org/dist/${_version}/${_file}"
    curl -fsSL "https://nodejs.org/dist/${_version}/SHASUMS256.txt" | grep " ${_file}\$" | (cd "${_work}" && sha256sum -c -)
    mkdir -p "${_work}/node"
    tar -xJf "${_work}/${_file}" -C "${_work}/node" --strip-components 1
    npm install --global --silent --prefix "${_work}/node" pnpm yarn@1 bun
}

install_osv_scanner() {
    mkdir -p "${_work}/bin"
    curl -fsSL -o "${_work}/bin/osv-scanner" \
        "https://github.com/google/osv-scanner/releases/download/v${_OSV_SCANNER_VERSION}/osv-scanner_linux_amd64"
    echo "${_OSV_SCANNER_SHA256}  ${_work}/bin/osv-scanner" | sha256sum -c -
    chmod +x "${_work}/bin/osv-scanner"
}

# A fixture copied to a scratch directory, for the tools to write in.
fixture() {
    local _dir
    _dir="$(mktemp -d)"
    cp -R "${_home}/tests/$1/." "${_dir}/"
    rm -f "${_dir}/package-lock.json"
    echo "${_dir}"
}

# save TOOL VERSION NAME: stdin to tests/output/TOOL/VERSION/NAME.
save() {
    mkdir -p "${_out}/$1/$2"
    cat >"${_out}/$1/$2/$3"
    echo "captured $1/$2/$3"
}

capture_node() {
    local _dir _pnpm _yarn _bun _npm
    _pnpm="$(pnpm --version)" _yarn="$(yarn --version)" _bun="$(bun --version)" _npm="$(npm --version)"

    _dir="$(fixture audit/fixtures/typescript/npm)"
    (cd "${_dir}" && pnpm install --lockfile-only --ignore-scripts >/dev/null && pnpm audit --json || true) |
        save pnpm "${_pnpm}" audit-pnpm-lodash.json
    _dir="$(fixture audit/fixtures/typescript/npm)"
    (cd "${_dir}" && yarn install --ignore-scripts --silent >/dev/null && yarn audit --json || true) |
        save yarn "${_yarn}" audit-yarn-classic-lodash.json
    _dir="$(fixture audit/fixtures/typescript/npm)"
    (cd "${_dir}" && bun install --ignore-scripts >/dev/null 2>&1 && bun audit --json || true) |
        save bun "${_bun}" audit-bun-lodash.json
    _dir="$(fixture audit/fixtures/typescript/npm)"
    (cd "${_dir}" && npm install --package-lock-only --ignore-scripts --no-audit --no-fund >/dev/null &&
        npm audit --json || true) | save npm "${_npm}" audit-npm-lodash.json
    _dir="$(fixture audit/fixtures/typescript/npm)"
    (cd "${_dir}" && yarn set version berry >/dev/null && YARN_ENABLE_IMMUTABLE_INSTALLS=false yarn install --mode=update-lockfile >/dev/null &&
        local _berry && _berry="$(yarn --version)" && { yarn npm audit --all --recursive --json || true; } >"${_work}/berry.json" &&
        echo "${_berry}" >"${_work}/berry.version")
    save yarn "$(cat "${_work}/berry.version")" audit-yarn-berry-lodash.json <"${_work}/berry.json"

    _dir="$(fixture deps/fixtures/typescript/npm)"
    (cd "${_dir}" && pnpm install --ignore-scripts >/dev/null && pnpm outdated --format json || true) |
        save pnpm "${_pnpm}" outdated-pnpm-left-pad.json
    _dir="$(fixture deps/fixtures/typescript/npm)"
    (cd "${_dir}" && yarn install --ignore-scripts --silent >/dev/null && yarn outdated --json || true) |
        save yarn "${_yarn}" outdated-yarn-classic-left-pad.json
    _dir="$(fixture deps/fixtures/typescript/npm)"
    (cd "${_dir}" && npm install --ignore-scripts --no-audit --no-fund >/dev/null && npm outdated --json || true) |
        save npm "${_npm}" outdated-npm-left-pad.json
}

capture_uv() {
    local _dir _uv
    curl -LsSf https://astral.sh/uv/install.sh | env UV_NO_MODIFY_PATH=1 sh >/dev/null
    _uv="$(uv --version | cut -d' ' -f2)"
    _dir="$(mktemp -d)"
    (cd "${_dir}" && uv venv --quiet && uv pip install --quiet six==1.15.0 && uv pip list --outdated --format json) |
        save uv "${_uv}" outdated-uv-pip-six.json
}

# OSV-Scanner on each audit fixture (the lock files and manifests it reads).
capture_osv() {
    local _fixture _lang _name
    for _fixture in "${_home}"/tests/audit/fixtures/*/*; do
        _lang="$(basename "$(dirname "${_fixture}")")"
        _name="$(basename "${_fixture}")"
        (cd "${_fixture}" && osv-scanner scan source --recursive --format json . 2>/dev/null || true) |
            save osv-scanner "${_OSV_SCANNER_VERSION}" "audit-osv-${_lang}-${_name}.json"
    done
}

install_osv_scanner
capture_osv
install_node
capture_node
capture_uv
