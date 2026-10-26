#!/usr/bin/env bash
# Check that every library's bundle is installed and loads.
source "$(dirname "$0")/../_common.sh"

while IFS= read -r _dir; do
    (cd "${_dir}" && bundle check >/dev/null 2>&1) || problem "${_dir#"${ROOT}/"}: bundle not installed"
done < <(impl_dirs ruby)
doctor_done
