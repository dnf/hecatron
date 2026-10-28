#!/usr/bin/env bash
# Check for what runs clang-format where a component configures it (see
# install.sh).
source "$(dirname "$0")/../_common.sh"

if meson_missing cpp; then
    problem "meson not installed (a component builds with it)"
fi
if clang_format_missing cpp; then
    problem "neither uv nor clang-format installed (a component has a .clang-format)"
fi
doctor_done
