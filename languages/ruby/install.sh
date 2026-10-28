#!/usr/bin/env bash
# Install Ruby: the default version and every version the project's
# components need, side by side, built by ruby-build (distributions' Ruby
# lags: Ubuntu 24.04 has 3.2, and many gems now need 3.3 or later). Each
# version ships its own Bundler.
source "$(dirname "$0")/../_common.sh"

# One directory per version. macOS's is per-user and needs no sudo.
if [ "${OS}" = "macos" ]; then
    _ruby_root="${HOME}/.local/share/hecatron/ruby"
    _ruby_build_dir="${HOME}/.local/share/hecatron/ruby-build"
    _sudo=""
else
    _ruby_root="/opt/ruby"
    _ruby_build_dir="/opt/ruby-build"
    _sudo="${SUDO}"
fi

# ruby-build from Homebrew, which brings the OpenSSL and libyaml it builds
# against.
install_macos() {
    has ruby-build || brew_install ruby-build
    _install_default
}
# ruby-build from its repository (distributions' copies lag, and know only
# the versions released before them), and the libraries Ruby builds against.
# Gems with native extensions compile with the same C toolchain.
install_linux() {
    apt_install build-essential autoconf git libssl-dev libyaml-dev zlib1g-dev libffi-dev libgmp-dev libreadline-dev
    if [ ! -x "${_ruby_build_dir}/bin/ruby-build" ]; then
        $SUDO rm -rf "${_ruby_build_dir}"
        $SUDO git clone --quiet --depth 1 https://github.com/rbenv/ruby-build.git "${_ruby_build_dir}"
    fi
    _install_default
    # The default version on every shell's PATH, when there's no Ruby at all.
    if ! has ruby; then
        local tool
        for tool in "${_ruby_root}/$(default_version)"/bin/*; do
            $SUDO ln -sfn "${tool}" "/usr/local/bin/$(basename "${tool}")"
        done
    fi
}

_ruby_build_path() {
    if [ -x "${_ruby_build_dir}/bin/ruby-build" ]; then echo "${_ruby_build_dir}/bin/ruby-build"; else command -v ruby-build; fi
}
# The releases ruby-build knows how to build. A Ruby released after this
# ruby-build has no definition yet, so update it first. Homebrew's formula
# can lag a Ruby release by weeks (its 20261013 has no 4.0.9), so on macOS
# that means ruby-build's repository, as on Linux, in a per-user directory;
# Homebrew's still brings the libraries it builds against.
_definitions() { "$(_ruby_build_path)" --definitions | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$'; }
_update_ruby_build() {
    log "Updating ruby-build, which has no definition of Ruby $1..."
    if [ -d "${_ruby_build_dir}/.git" ]; then
        ${_sudo} git -C "${_ruby_build_dir}" fetch --quiet --depth 1 origin
        ${_sudo} git -C "${_ruby_build_dir}" reset --quiet --hard FETCH_HEAD
    else
        ${_sudo} rm -rf "${_ruby_build_dir}"
        ${_sudo} mkdir -p "$(dirname "${_ruby_build_dir}")"
        ${_sudo} git clone --quiet --depth 1 https://github.com/rbenv/ruby-build.git "${_ruby_build_dir}"
    fi
}
_install_ruby() {
    local version
    version="$(_definitions | newest_match "$1")"
    if [ -z "${version}" ]; then
        _update_ruby_build "${1#=}"
        version="$(_definitions | newest_match "$1")"
    fi
    if [ -z "${version}" ]; then
        echo "No Ruby release matches ${1#=}" >&2
        return 1
    fi
    [ -x "${_ruby_root}/${version}/bin/ruby" ] && return
    log "Building Ruby ${version} with ruby-build (a few minutes)..."
    ${_sudo} mkdir -p "${_ruby_root}"
    # Without the documentation, which no build or test reads.
    ${_sudo} env RUBY_CONFIGURE_OPTS=--disable-install-doc PATH="${PATH}" \
        "$(_ruby_build_path)" "${version}" "${_ruby_root}/${version}" </dev/null
}
# Outside a project (or when the project's components all use another
# version), the default still gets installed.
_install_default() { _install_ruby "=$(default_version)"; }

_install_versions() {
    local specs spec
    specs="$(missing_versions ruby)"
    while IFS= read -r spec; do
        [ -n "${spec}" ] || continue
        _install_ruby "${spec}"
    done <<<"${specs}"
}

# A Ruby built into a root-owned directory keeps gems where `bundle install`
# can't write: send bundles to the user's home instead (in Bundler's global
# config, so `bundle exec` finds them there too). Bundler keeps each Ruby
# version's gems apart under it.
_bundle_user_path() {
    bundle_needs_user_path || return 0
    log "Installing bundles under ${HOME}/.local/share/gem (the Ruby's gem directory is read-only)"
    bundle config set --global path "${HOME}/.local/share/gem"
}
# Only libraries with a Gemfile have a bundle to install; each with its own
# Ruby, since native extensions are built for one.
deps() {
    local default_bin
    _install_versions
    default_bin="${_ruby_root}/$(default_version)/bin"
    export PATH="${default_bin}:${PATH}"
    _bundle_user_path
    _bundler_audit
    impl_deps ruby
}

# bundler-audit, the dependency auditor hecatron audit uses for Ruby, in the
# default Ruby and linked onto PATH, so a component on another Ruby finds it.
_BUNDLER_AUDIT_VERSION="0.9.3"
_bundler_audit() {
    local default_bin
    default_bin="${_ruby_root}/$(default_version)/bin"
    if [ ! -x "${default_bin}/bundle-audit" ]; then
        log "Installing bundler-audit ${_BUNDLER_AUDIT_VERSION} (for hecatron audit)..."
        ${_sudo} "${default_bin}/gem" install --no-document bundler-audit -v "${_BUNDLER_AUDIT_VERSION}"
    fi
    has bundle-audit || { mkdir -p "$(hecatron_bin)" && ln -sfn "${default_bin}/bundle-audit" "$(hecatron_bin)/bundle-audit"; }
}

log "Installing Ruby..."
"install_${OS}"
deps
# OSV-Scanner, which hecatron licenses reads this language's licenses with.
osv_scanner_install
