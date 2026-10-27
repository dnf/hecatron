#!/usr/bin/env bash
# Check install.sh (hecatron's own installer) in a throwaway container, as a
# normal user, installing from the mounted checkout instead of GitHub:
#   tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/installer/check.sh
# It edits the user's shell startup files, so never run it on your machine.
set -u
_failed=0
sudo apt-get update -qq >/dev/null 2>&1; sudo apt-get install -y -qq zsh fish >/dev/null 2>&1
export HECATRON_REPO=/hecatron
git config --global --add safe.directory "*"
# The test image links hecatron into /usr/local/bin; take that away so only an install can provide it.
sudo rm -f /usr/local/bin/hecatron /usr/local/bin/hct
check() { if eval "$2" >/dev/null 2>&1; then echo "PASS $1"; else echo "FAIL $1"; _failed=1; fi; }
echo "== 1. curl | sh (dash)"
sh < /hecatron/install.sh | sed 's/^/   /'
check "installed into .hecatron" '[ -x ~/.hecatron/bin/hecatron ]'
check ".profile sources env" 'grep -qF ".hecatron/env" ~/.profile'
check ".bashrc sources env" 'grep -qF ".hecatron/env" ~/.bashrc'
check "new login bash finds hct" 'bash -lc "command -v hct" | grep -q .hecatron/bin'
check "new interactive bash finds hct" 'bash -ic "command -v hct" 2>/dev/null | grep -q .hecatron/bin'
check "fish finds hct" 'fish -c "command -v hct" | grep -q .hecatron/bin'
echo "== 2. rerun"
sh < /hecatron/install.sh >/dev/null 2>&1
check "one line in ~/.profile" '[ "$(grep -cF ".hecatron/env" ~/.profile)" = 1 ]'
check "one line in ~/.bashrc" '[ "$(grep -cF ".hecatron/env" ~/.bashrc)" = 1 ]'
echo "== 3. zsh user with no startup file"
SHELL=/bin/zsh sh < /hecatron/install.sh >/dev/null 2>&1
check ".zshrc created" 'grep -qF ".hecatron/env" ~/.zshrc'
check "new zsh finds hct" 'zsh -ic "command -v hct" | grep -q .hecatron/bin'
echo "== 4. sourced from a clone, bash"
git clone -q /hecatron ~/src/hecatron
out="$(env -u HECATRON_REPO bash -c 'PATH=/usr/bin:/bin; . ~/src/hecatron/install.sh; command -v hct; echo alive')"
echo "$out" | sed 's/^/   /'
check "this shell has hct, from the clone" 'echo "$out" | grep -q "src/hecatron/bin/hct"'
check "the shell survives" 'echo "$out" | grep -q "^alive$"'
check "profile points at the clone" 'grep -qF "src/hecatron/env" ~/.profile'
check "and only at the clone" '[ "$(grep -c "  # hecatron$" ~/.profile)" = 1 ]'
echo "== 5. sourced from a clone, zsh"
out="$(env -u HECATRON_REPO zsh -c 'PATH=/usr/bin:/bin; . ~/src/hecatron/install.sh; command -v hct; echo alive')"
check "zsh shell has hct" 'echo "$out" | grep -q "src/hecatron/bin/hct"'
check "zsh shell survives" 'echo "$out" | grep -q "^alive$"'
echo "== 6. a failure while sourced leaves the shell running"
mkdir -p ~/notaclone
out="$(bash -c 'HECATRON_INSTALL_DIR=~/notaclone; export HECATRON_INSTALL_DIR; . /dev/stdin < /hecatron/install.sh; echo "status=$? alive"' 2>&1)"
echo "$out" | sed 's/^/   /'
check "error reported and shell survives" 'echo "$out" | grep -q "isn.t a hecatron clone" && echo "$out" | grep -q "alive"'
echo "== 7. HECATRON_NO_MODIFY_PATH"
cp ~/.profile ~/.profile.before
HECATRON_NO_MODIFY_PATH=1 HECATRON_INSTALL_DIR=~/other sh < /hecatron/install.sh >/dev/null 2>&1
check "profile untouched" 'cmp -s ~/.profile ~/.profile.before'
check "still installed" '[ -x ~/other/bin/hecatron ]'
exit "${_failed}"
