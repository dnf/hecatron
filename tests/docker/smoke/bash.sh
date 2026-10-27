# Smoke test for bash: run a script with the installed bash, and lint it with ShellCheck.
# shellcheck shell=bash
set -euo pipefail
cat > hello.sh <<'EOF'
#!/usr/bin/env bash
squares=(0 1 4 9 16 25 36)
echo "$(( squares[6] + 6 ))"
EOF
_out="$(bash hello.sh)"
[ "${_out}" = "42" ] || { echo "expected 42, got: ${_out}" >&2; exit 1; }
echo "${_out}"
shellcheck -x hello.sh
