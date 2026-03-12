#!/usr/bin/env bash
# Uninstall Pharo Smalltalk
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
	brew uninstall pharo 2>/dev/null || true
	rm -rf "${HOME}/.pharo" 2>/dev/null || true
	rm -f /usr/local/bin/pharo 2>/dev/null || true
}

uninstall_linux() {
	rm -rf "${HOME}/.pharo" 2>/dev/null || true
	rm -f /usr/local/bin/pharo 2>/dev/null || true
}

log "Uninstalling Pharo..."
"uninstall_${OS}"
