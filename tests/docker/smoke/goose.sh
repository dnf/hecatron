# Smoke test for goose: run and build a hello world in a scratch directory, and
# check it with goose check as the default lint hook does.
# shellcheck shell=bash
set -euo pipefail
cat > hello.goose <<'EOF'
fn add(a: i32, b: i32) -> i32 {
    return a + b;
}

fn main() {
    assert(add(40, 2) == 42);
    print("hello");
}
EOF
_out="$(goose run hello.goose)"
[ "${_out}" = "hello" ] || { echo "expected hello, got: ${_out}" >&2; exit 1; }
echo "${_out}"
goose build hello.goose -o hello
_out="$(./hello)"
[ "${_out}" = "hello" ] || { echo "expected hello from the build, got: ${_out}" >&2; exit 1; }
_diagnostics="$(goose check --diagnostics json hello.goose)"
[ "${_diagnostics}" = "[]" ] || { echo "goose check: ${_diagnostics}" >&2; exit 1; }
