#!/usr/bin/env bash
# Check for fpm where a component builds with an fpm.toml (see install.sh).
source "$(dirname "$0")/../_common.sh"

if ! has fpm; then
    while IFS= read -r _dir; do
        if [ -f "${_dir}/fpm.toml" ]; then
            problem "fpm not installed (${_dir#"${ROOT}/"} builds with fpm.toml)"
            break
        fi
    done < <(impl_dirs fortran)
fi
doctor_done
