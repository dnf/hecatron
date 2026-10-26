#!/usr/bin/env bash
# Check that every library's bundle is installed and loads.
source "$(dirname "$0")/../_common.sh"

while IFS= read -r _dir; do
    [ -f "${_dir}/Gemfile" ] || continue
    (cd "${_dir}" && bundle check </dev/null >/dev/null 2>&1) || problem "${_dir#"${ROOT}/"}: bundle not installed"
done < <(impl_dirs ruby)
# Without Ruby's headers, any gem with a native extension fails to install.
if has ruby && ! ruby -e 'exit File.exist?(File.join(RbConfig::CONFIG["rubyhdrdir"], "ruby.h"))'; then
    problem "Ruby's headers are missing, so gems with native extensions can't build"
fi
doctor_done
