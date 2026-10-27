#!/usr/bin/env bash
# Install Node.js (JavaScript) — shares Node.js with TypeScript
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install node; }
# Node.js is TypeScript's; its installer knows where Linux gets it.
install_linux() { bash "$(dirname "$0")/../typescript/install.sh"; }

# Only packages with a package.json have dependencies to install.
_npm_install() { [ ! -f package.json ] || npm install; }
deps() { each_impl javascript _npm_install; }

log "Installing JavaScript (Node.js)..."
"install_${OS}"
deps
