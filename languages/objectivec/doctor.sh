#!/usr/bin/env bash
# Check for the Objective-C compiler and Foundation: on Linux, GCC's
# Objective-C front end and GNUstep (gnustep-config, which gives the flags
# and GNUSTEP_MAKEFILES); on macOS, Xcode's clang. And CMake where a
# component builds with it (see install.sh).
source "$(dirname "$0")/../_common.sh"

_gcc_objc() { has gcc && [ -x "$(gcc -print-prog-name=cc1obj)" ]; }
if [ "${OS}" = "linux" ]; then
    _gcc_objc || problem "GCC's Objective-C compiler (gobjc) not installed"
    has gnustep-config || problem "gnustep-config not found (GNUstep Base and gnustep-make)"
else
    xcrun --find clang >/dev/null 2>&1 || problem "Xcode's clang not found (xcode-select --install)"
fi
_dir=""
if ! has cmake; then
    while IFS= read -r _dir; do
        [ -f "${_dir}/CMakeLists.txt" ] && { problem "cmake not installed (a component builds with it)"; break; }
    done < <(impl_dirs objectivec)
fi
if meson_missing objectivec; then
    problem "meson not installed (a component builds with it)"
fi
if clang_format_missing objectivec; then
    problem "neither uv nor clang-format installed (a component has a .clang-format)"
fi
doctor_done
