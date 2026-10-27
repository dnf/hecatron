#!/usr/bin/env bash
# Uninstall Java (Temurin JDKs)
source "$(dirname "$0")/../_common.sh"

_message() {
    echo "JDKs are shared by Java, Kotlin, Scala, and Groovy, and by other projects — not removing."
    echo "To force, remove the JDK directories: $1"
}
uninstall_macos() { _message "${HOME}/Library/Java/JavaVirtualMachines/temurin-*"; }
uninstall_linux() { _message "/usr/lib/jvm/temurin-* (and their links in /usr/local/bin)"; }

log "Uninstalling Java..."
"uninstall_${OS}"
