#!/usr/bin/env bash
# Install Ruby
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install ruby; }
install_linux() { apt_install ruby ruby-bundler; }

# Only libraries with a Gemfile have a bundle to install.
_bundle_install() { [ ! -f Gemfile ] || bundle install; }
deps() { each_impl ruby _bundle_install; }

log "Installing Ruby..."
"install_${OS}"
deps
