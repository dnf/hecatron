#!/usr/bin/env bash
# Install Pharo Smalltalk
source "$(dirname "$0")/../_common.sh"

PHARO_VERSION="120"

install_macos() {
	if has pharo; then
		log "Pharo already installed"
		return
	fi
	brew install pharo 2>/dev/null || {
		log "Installing Pharo via zeroconf..."
		mkdir -p "${HOME}/.pharo"
		cd "${HOME}/.pharo"
		curl -fsSL https://get.pharo.org/64/${PHARO_VERSION}+vm | bash
		ln -sf "${HOME}/.pharo/pharo" /usr/local/bin/pharo 2>/dev/null || true
	}
}

install_linux() {
	if has pharo; then
		log "Pharo already installed"
		return
	fi
	log "Installing Pharo via zeroconf..."
	mkdir -p "${HOME}/.pharo"
	cd "${HOME}/.pharo"
	curl -fsSL https://get.pharo.org/64/${PHARO_VERSION}+vm | bash
	ln -sf "${HOME}/.pharo/pharo" /usr/local/bin/pharo 2>/dev/null || true
}

deps() { :; }

log "Installing Pharo..."
"install_${OS}"
deps
