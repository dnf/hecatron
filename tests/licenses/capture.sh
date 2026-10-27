#!/usr/bin/env bash
# Captures OSV-Scanner's license report (every package, --all-packages) on each fixture project in
# tests/licenses/fixtures/<lang>/ (lock files with dependencies of known
# licenses), as hct licenses runs it, into
# tests/output/osv-scanner/<version>/licenses-<lang>.json under CAPTURE_OUT
# (default: tests/output); internal/licenses' tests read those offline.
# OSV-Scanner asks deps.dev for the licenses, so it needs the network; it's
# downloaded at the version hecatron installs. Run it in a throwaway
# container:
#
#   docker run --rm -v "$PWD:/hecatron:ro" -v "$PWD/tests/output:/out" \
#       -e CAPTURE_OUT=/out hecatron-test:ubuntu-24.04 bash /hecatron/tests/licenses/capture.sh
set -euo pipefail

_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_out="${CAPTURE_OUT:-${_home}/tests/output}"
_OSV_SCANNER_VERSION="2.7.0"
_OSV_SCANNER_SHA256="160c6253e2922e9e0a6a9771c5c4c40a4b3c6090b34a3cdd5e80c96cc2e4218b"
_work="$(mktemp -d)"

curl -fsSL -o "${_work}/osv-scanner" \
    "https://github.com/google/osv-scanner/releases/download/v${_OSV_SCANNER_VERSION}/osv-scanner_linux_amd64"
echo "${_OSV_SCANNER_SHA256}  ${_work}/osv-scanner" | sha256sum -c -
chmod +x "${_work}/osv-scanner"

# The fixtures at a fixed path (/fixtures/<lang>), as the report names them.
_fixtures=/tmp/licenses-fixtures
rm -rf "${_fixtures}"
cp -R "${_home}/tests/licenses/fixtures" "${_fixtures}"
mkdir -p "${_out}/osv-scanner/${_OSV_SCANNER_VERSION}"
for _dir in "${_fixtures}"/*; do
    _lang="$(basename "${_dir}")"
    _status=0
    (cd "${_dir}" && "${_work}/osv-scanner" scan source --recursive --all-packages --licenses --format json .) \
        >"${_out}/osv-scanner/${_OSV_SCANNER_VERSION}/licenses-${_lang}.json" 2>/dev/null || _status=$?
    echo "captured licenses-${_lang}.json (status ${_status})"
done
