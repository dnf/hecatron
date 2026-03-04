#!/usr/bin/env bash
# Install OpenJDK 17 (shared by Java, Kotlin, Scala, Groovy)
source "$(dirname "$0")/../_common.sh"

install_macos() {
    brew_install openjdk@17
    # Make Java 17 available to /usr/libexec/java_home
    OPENJDK_PATH="$(brew --prefix openjdk@17)/libexec/openjdk.jdk"
    JVM_DIR="/Library/Java/JavaVirtualMachines/openjdk-17.jdk"
    if [ -d "$OPENJDK_PATH" ] && [ ! -e "$JVM_DIR" ]; then
        log "Symlinking OpenJDK 17 (requires sudo)..."
        $SUDO ln -sfn "$OPENJDK_PATH" "$JVM_DIR" || \
            echo "   Skipped (run manually: sudo ln -sfn $OPENJDK_PATH $JVM_DIR)"
    fi
}

install_linux() { apt_install openjdk-17-jdk-headless; }

deps() { :; }

log "Installing Java (OpenJDK 17)..."
"install_${OS}"
deps
