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

# The distribution's OpenJDK 17, else Eclipse Temurin 17 from Adoptium's
# repository, for distributions that no longer package 17 (Debian 13).
install_linux() {
    if apt_has openjdk-17-jdk-headless; then
        apt_install openjdk-17-jdk-headless
    else
        _add_adoptium_source
        apt_install temurin-17-jdk
    fi
}
_add_adoptium_source() {
    local keyring="/etc/apt/keyrings/adoptium.asc" codename
    [ -f /etc/apt/sources.list.d/adoptium.list ] && return
    codename="$(. /etc/os-release && echo "${VERSION_CODENAME}")"
    log "Adding Adoptium's apt repository for ${codename}..."
    $SUDO install -d -m 0755 "$(dirname "${keyring}")"
    curl -fsSL https://packages.adoptium.net/artifactory/api/gpg/key/public | $SUDO tee "${keyring}" >/dev/null
    echo "deb [signed-by=${keyring}] https://packages.adoptium.net/artifactory/deb ${codename} main" |
        $SUDO tee /etc/apt/sources.list.d/adoptium.list >/dev/null
    $SUDO apt-get update -qq
}

deps() { :; }

log "Installing Java (OpenJDK 17)..."
"install_${OS}"
deps
