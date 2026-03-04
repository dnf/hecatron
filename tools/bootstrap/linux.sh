#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]:-$0}")/.."

echo "==> parent-time: Linux development setup (Ubuntu/Debian)"
echo ""

# Use sudo only when not already root
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    SUDO="sudo"
fi

# ---- Base packages ----
echo "==> Installing base packages..."
$SUDO apt-get update
$SUDO apt-get install -y curl ca-certificates git gnupg unzip

# ---- just (task runner) ----
if ! command -v just &>/dev/null; then
    echo "==> Installing just..."
    curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | $SUDO bash -s -- --to /usr/local/bin
fi

# ---- Install all language toolchains ----
just install

# ---- Persist PATH for future shells ----
SHELL_RC="$HOME/.bashrc"
[ -n "${ZSH_VERSION:-}" ] && SHELL_RC="$HOME/.zshrc"
{
    echo ''
    echo '# Added by parent-time bootstrap'
    echo 'export PATH="/usr/local/go/bin:$PATH"'
    echo '[ -f "$HOME/.cargo/env" ] && . "$HOME/.cargo/env"'
    echo '[ -f "$HOME/.ghcup/env" ] && . "$HOME/.ghcup/env"'
    echo 'export PATH="$HOME/.local/bin:$PATH"'
} >> "$SHELL_RC"

echo ""
echo "==> Setup complete! Verify with: just test"
echo ""
echo "PATH entries were added to $SHELL_RC."
echo "Run 'source $SHELL_RC' or open a new terminal, then run 'just test'."
