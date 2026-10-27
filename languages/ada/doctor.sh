#!/usr/bin/env bash
# Check for Alire where a component is a crate (alire.toml; see install.sh).
source "$(dirname "$0")/../_common.sh"

if ! has alr; then
    while IFS= read -r _dir; do
        if [ -f "${_dir}/alire.toml" ]; then
            problem "alr not installed (${_dir#"${ROOT}/"} is an Alire crate)"
            break
        fi
    done < <(impl_dirs ada)
fi
doctor_done
