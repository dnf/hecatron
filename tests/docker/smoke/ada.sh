# Smoke test for ada: build a hello world with GPRbuild and run it in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'project Hello is\n   for Main use ("hello.adb");\nend Hello;\n' > hello.gpr
cat > hello.adb <<'EOF'
with Ada.Text_IO;
procedure Hello is
begin
   Ada.Text_IO.Put_Line (Integer'Image (6 * 7));
end Hello;
EOF
gprbuild -q -P hello.gpr -cargs -gnatwae
_out="$(./hello)"
[ "${_out}" = " 42" ] || { echo "expected ' 42', got: '${_out}'" >&2; exit 1; }
echo "${_out}"
