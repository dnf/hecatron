#!/usr/bin/env bash
# Install Ruby
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install ruby; }
# Gems with native extensions (json, racc...) compile against ruby-dev's
# headers with the C toolchain, which a fresh machine lacks.
install_linux() {
    apt_install ruby ruby-dev ruby-bundler
    { has make && has cc; } || apt_install build-essential
}

# Only libraries with a Gemfile have a bundle to install.
_bundle_install() { [ ! -f Gemfile ] || bundle install; }
deps() { each_impl ruby _bundle_install; }

log "Installing Ruby..."
"install_${OS}"
deps
