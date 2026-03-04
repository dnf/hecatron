#!/usr/bin/env bash
# Uninstall Kotlin
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    echo "JDK 17 is shared by Java, Kotlin, Scala, and Groovy — not removing."
    echo "To force: brew uninstall openjdk@17"
}

uninstall_linux() {
    echo "JDK 17 is shared by Java, Kotlin, Scala, and Groovy — not removing."
    echo "To force: sudo apt-get remove -y openjdk-17-jdk-headless"
}

log "Uninstalling Kotlin..."
"uninstall_${OS}"
