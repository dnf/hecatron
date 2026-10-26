#!/usr/bin/env bash
# hecatron bootstrap: set up this machine for the project and install every
# language toolchain it uses.
set -euo pipefail

cd "${HECATRON_ROOT}"

echo "==> ${HECATRON_PROJECT}: macOS development setup"
echo ""

# ---- Homebrew ----
if ! command -v brew &>/dev/null; then
    echo "==> Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# ---- Install all language toolchains ----
"${HECATRON_HOME}/bin/hecatron" install

echo ""
echo "==> Setup complete! Verify with: hecatron test"
