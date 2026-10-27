#!/usr/bin/env bash
# Check that rustc actually runs: an interrupted rustup install leaves its
# proxy on PATH with no toolchain behind it (see install.sh).
source "$(dirname "$0")/../_common.sh"

if has rustc && ! rustc --version >/dev/null 2>&1; then
    problem "rustc doesn't run: rustup has no default toolchain"
fi
doctor_done
