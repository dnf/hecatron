#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]:-$0}")/.."

echo "==> parent-time: macOS development setup"
echo ""

# ---- Homebrew ----
if ! command -v brew &>/dev/null; then
    echo "==> Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# ---- just (task runner) ----
if ! command -v just &>/dev/null; then
    echo "==> Installing just..."
    brew install just
fi

# ---- Install all language toolchains ----
just install

echo ""
echo "==> Setup complete! Verify with: just test"
