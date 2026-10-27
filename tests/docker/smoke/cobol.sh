# Smoke test for cobol: build and run a hello world with GnuCOBOL in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
cat > hello.cob <<'EOF'
       IDENTIFICATION DIVISION.
       PROGRAM-ID. HELLO.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01 ANSWER PIC 99.
       PROCEDURE DIVISION.
           COMPUTE ANSWER = 6 * 7.
           DISPLAY ANSWER.
           STOP RUN.
EOF
cobc -x -Wall -o hello hello.cob
_out="$(./hello)"
[ "${_out}" = "42" ] || { echo "expected 42, got: ${_out}" >&2; exit 1; }
echo "${_out}"
cmake --version | head -1
