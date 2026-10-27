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

# apt's Ruby keeps gems in a root-owned directory, which `bundle install`
# can't write to: send bundles to the user's home instead (in Bundler's
# global config, so `bundle exec` finds them there too).
_bundle_user_path() {
    bundle_needs_user_path || return 0
    log "Installing bundles under ${HOME}/.local/share/gem (the system gem directory is read-only)"
    bundle config set --global path "${HOME}/.local/share/gem"
}
# Only libraries with a Gemfile have a bundle to install.
_bundle_install() { [ ! -f Gemfile ] || bundle install; }
deps() {
    _bundle_user_path
    each_impl ruby _bundle_install
}

log "Installing Ruby..."
"install_${OS}"
deps
