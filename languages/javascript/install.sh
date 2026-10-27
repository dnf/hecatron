#!/usr/bin/env bash
# Install Node.js (JavaScript) — shares Node.js with TypeScript
source "$(dirname "$0")/../_common.sh"
# shellcheck source=languages/typescript/package-manager.sh
source "$(dirname "$0")/../typescript/package-manager.sh"

install_macos() { brew_install node; }
# Node.js is TypeScript's; its installer knows where Linux gets it.
install_linux() { bash "$(dirname "$0")/../typescript/install.sh"; }

# The package manager each package declares, which its dependencies (hecatron
# deps) are then installed with.
_node_package_manager() {
    [ -f package.json ] || return 0
    node_package_manager_ensure
}
deps() {
    each_impl javascript _node_package_manager
    impl_deps javascript
}

log "Installing JavaScript (Node.js)..."
"install_${OS}"
deps
