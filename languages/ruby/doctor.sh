#!/usr/bin/env bash
# Check that every library's bundle is installed and loads.
source "$(dirname "$0")/../_common.sh"

while IFS= read -r _dir; do
    [ -f "${_dir}/Gemfile" ] || continue
    if ! has bundle; then
        problem "Bundler isn't installed"
        break
    fi
    (cd "${_dir}" && bundle check </dev/null >/dev/null 2>&1) ||
        problem "${_dir#"${ROOT}/"}: the Gemfile's gems aren't installed (hecatron deps ruby installs them)"
done < <(impl_dirs ruby)
if bundle_needs_user_path; then
    problem "bundle install can't write to the system gem directory and Bundler has no path configured"
fi
# Without Ruby's headers, any gem with a native extension fails to install.
if has ruby && ! ruby -e 'exit File.exist?(File.join(RbConfig::CONFIG["rubyhdrdir"], "ruby.h"))'; then
    problem "Ruby's headers are missing, so gems with native extensions can't build"
fi
doctor_done
