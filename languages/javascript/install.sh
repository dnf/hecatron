#!/usr/bin/env bash
# Install Node.js (JavaScript) — shares Node.js with TypeScript
source "$(dirname "$0")/../_common.sh"
# shellcheck source=languages/typescript/package-manager.sh
source "$(dirname "$0")/../typescript/package-manager.sh"

# Node.js is TypeScript's; its installer knows where each OS gets it, and
# installs the versions JavaScript's components resolve too.
install_node() { bash "$(dirname "$0")/../typescript/install.sh"; }

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
install_node
deps
# OSV-Scanner, which hecatron licenses reads this language's licenses with.
osv_scanner_install
