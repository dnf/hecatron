#!/usr/bin/env bash
# Check that each component's PHP satisfies what its locked dependencies
# require of PHP (a lock resolved on PHP 8.4 doesn't install on 8.3). The
# version comes from the component's pin file or the default; this catches a
# default older than the lock needs.
source "$(dirname "$0")/../_common.sh"

[ -n "${HECATRON_HOME:-}" ] && has composer || doctor_done
_project() { (cd "${ROOT}" && python3 "${HECATRON_HOME}/libexec/lib/project.py" "$@" </dev/null); }
# Composer's check, for PHP itself only: an extension it lacks is the
# project's to install.
_php_unsatisfied() {
    composer check-platform-reqs --lock --format=json 2>/dev/null | python3 -c '
import json, sys
try:
    reqs = json.load(sys.stdin)
except ValueError:
    sys.exit()
for req in reqs:
    failed = req.get("failed_requirement") or {}
    if req.get("name") == "php" and req.get("status") != "success" and failed:
        print("PHP %s does not satisfy %s %s" % (req.get("version"), failed.get("source"), failed.get("constraint")))
'
}

while IFS=$'\t' read -r _component _dir; do
    [ -f "${_dir}/composer.lock" ] || continue
    # Composer exits non-zero when a requirement fails; the message says which.
    _found="$(eval "$(_project env php "${_component}")" && cd "${_dir}" && _php_unsatisfied | head -1)" || true
    [ -z "${_found}" ] || problem "${_dir#"${ROOT}/"}: ${_found} (choose a version with .php-version or composer.json's config.platform.php)"
done < <(_project impl-dirs php)
doctor_done
