#!/usr/bin/env bash
# Install Scala: the JDK (Java's installer) and Scala's own compiler, so a
# project can build Scala with or without a build tool (Scala 3, the LTS line).
source "$(dirname "$0")/../_common.sh"

_version="3.3.8"

_jdk() { has java || bash "$(dirname "$0")/../java/install.sh"; }

install_macos() {
    _jdk
    has scalac || brew_install scala
}
install_linux() {
    _jdk
    has scalac || jvm_language_install scala "${_version}" \
        "https://github.com/scala/scala3/releases/download/${_version}/scala3-${_version}.tar.gz" scala scalac
}

deps() { jvm_build_tools scala; }

log "Installing Scala..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install
