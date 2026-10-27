#!/usr/bin/env bash
# Install Kotlin: the JDK (Java's installer) and Kotlin's own compiler, so a
# project can build Kotlin with or without a build tool.
source "$(dirname "$0")/../_common.sh"

_version="2.4.21"

_jdk() { has java || bash "$(dirname "$0")/../java/install.sh"; }

install_macos() {
    _jdk
    has kotlinc || brew_install kotlin
}
install_linux() {
    _jdk
    has kotlinc || jvm_language_install kotlin "${_version}" \
        "https://github.com/JetBrains/kotlin/releases/download/v${_version}/kotlin-compiler-${_version}.zip" kotlinc kotlin
}

deps() { jvm_build_tools kotlin; }

log "Installing Kotlin..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install
