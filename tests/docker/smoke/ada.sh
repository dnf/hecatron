# TEMPORARY DIAGNOSTIC (branch only): why GNAT's link fails on macOS arm64.
# shellcheck shell=bash
set -uo pipefail
printf 'project Hello is\n   for Main use ("hello.adb");\nend Hello;\n' > hello.gpr
cat > hello.adb <<'ADA'
with Ada.Text_IO;
procedure Hello is
begin
   Ada.Text_IO.Put_Line (Integer'Image (6 * 7));
end Hello;
ADA
echo "## xcode-select: $(xcode-select -p)"
echo "## xcrun sdk: $(xcrun --show-sdk-path) ($(xcrun --show-sdk-version))"
echo "## ld: $(command -v ld) / $(ld -v 2>&1 | head -1)"
echo "## CLT SDKs: $(ls /Library/Developer/CommandLineTools/SDKs/ | tr '\n' ' ')"
echo "## CLT tbd:"; sed -n 1,5p /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/lib/libSystem.tbd
echo "## xcrun tbd:"; sed -n 1,5p "$(xcrun --show-sdk-path)/usr/lib/libSystem.tbd"
echo "## gcc -print-sysroot: $(gcc -print-sysroot 2>&1)"; gcc -v 2>&1 | tail -2
echo "## build as is"; gprbuild -q -P hello.gpr -cargs -gnatwae 2>&1 | tail -2; ./hello
rm -f hello hello.o b__hello* obj -r 2>/dev/null
echo "## build with SDKROOT=xcrun's"; SDKROOT="$(xcrun --show-sdk-path)" gprbuild -q -P hello.gpr -cargs -gnatwae 2>&1 | tail -2; ./hello
