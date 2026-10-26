#!/usr/bin/env bash
# hecatron bootstrap: set up this machine for the project and install every
# language toolchain it uses.
set -euo pipefail

cd "${HECATRON_ROOT}"

echo "==> ${HECATRON_PROJECT}: Linux development setup (Ubuntu/Debian)"
echo ""

# Use sudo only when not already root
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    SUDO="sudo"
fi

# ---- Base packages ----
echo "==> Installing base packages..."
$SUDO apt-get update
$SUDO apt-get install -y curl ca-certificates git gnupg perl unzip

# ---- Install all language toolchains ----
"${HECATRON_HOME}/bin/hecatron" install

# ---- Persist PATH for future shells ----
SHELL_RC="$HOME/.bashrc"
[ -n "${ZSH_VERSION:-}" ] && SHELL_RC="$HOME/.zshrc"
{
    echo ''
    echo "# Added by hecatron bootstrap (${HECATRON_PROJECT})"
    echo 'export PATH="/usr/local/go/bin:$PATH"'
    echo '[ -f "$HOME/.cargo/env" ] && . "$HOME/.cargo/env"'
    echo '[ -f "$HOME/.ghcup/env" ] && . "$HOME/.ghcup/env"'
    echo 'export PATH="$HOME/.local/bin:$PATH"'
} >> "$SHELL_RC"

echo ""
echo "==> Setup complete! Verify with: hecatron test"
echo ""
echo "PATH entries were added to $SHELL_RC."
echo "Run 'source $SHELL_RC' or open a new terminal, then run 'hecatron test'."
