#!/usr/bin/env bash
# Uninstall Kotlin's compiler. The JDK is shared by the JVM languages and
# other projects, so it stays (see Java's uninstall.sh).
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall kotlin 2>/dev/null || true; }
uninstall_linux() {
    local _tool
    for _tool in kotlinc kotlin; do
        case "$(readlink "/usr/local/bin/${_tool}" 2>/dev/null)" in
            /opt/kotlin/*) $SUDO rm -f "/usr/local/bin/${_tool}" ;;
        esac
    done
    $SUDO rm -rf "/opt/kotlin"
}

log "Uninstalling Kotlin..."
"uninstall_${OS}"
