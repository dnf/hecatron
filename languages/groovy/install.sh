#!/usr/bin/env bash
# Install Groovy: the JDK (Java's installer) and Groovy's own compiler, so a
# project can build Groovy with or without a build tool.
source "$(dirname "$0")/../_common.sh"

_version="5.1.3"

_jdk() { has java || bash "$(dirname "$0")/../java/install.sh"; }

install_macos() {
    _jdk
    has groovy || brew_install groovy
}
install_linux() {
    _jdk
    has groovy || jvm_language_install groovy "${_version}" \
        "https://repo1.maven.org/maven2/org/apache/groovy/groovy-binary/${_version}/groovy-binary-${_version}.zip" groovy groovyc
}

deps() { jvm_build_tools groovy; }

log "Installing Groovy..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install
