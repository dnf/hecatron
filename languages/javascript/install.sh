#!/usr/bin/env bash
# Install Node.js (JavaScript) — shares Node.js with TypeScript
source "$(dirname "$0")/../_common.sh"
# shellcheck source=languages/typescript/package-manager.sh
source "$(dirname "$0")/../typescript/package-manager.sh"

install_macos() { brew_install node; }
# Node.js is TypeScript's; its installer knows where Linux gets it.
install_linux() { bash "$(dirname "$0")/../typescript/install.sh"; }

# Only packages with a package.json have dependencies to install, with the
# package manager each declares.
_node_install() {
    [ -f package.json ] || return 0
    node_package_manager_ensure
    "${node_pm[@]}" install
}
deps() { each_impl javascript _node_install; }

log "Installing JavaScript (Node.js)..."
"install_${OS}"
deps
