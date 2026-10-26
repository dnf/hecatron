#!/usr/bin/env bash
# Check for virtualenvs moved from another checkout, whose scripts can't start.
source "$(dirname "$0")/../_common.sh"

while IFS= read -r _dir; do
    _script="${_dir}/.venv/bin/pytest"
    if [ -f "${_script}" ] && ! head -1 "${_script}" | grep -qF "${_dir}/.venv/"; then
        problem "${_dir#"${ROOT}/"}: .venv was moved from another checkout"
    fi
done < <(impl_dirs python)
doctor_done
