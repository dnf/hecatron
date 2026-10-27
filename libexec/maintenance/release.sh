#!/usr/bin/env bash
# Release hecatron: check, bump, merge, tag and publish, in one command.
#
#   libexec/maintenance/release.sh 0.3.0 --dry-run   # checks and the planned changes only
#   libexec/maintenance/release.sh 0.3.0
#
# 1. Checks: on main, clean and up to date with origin; gh signed in; the
#    version is X.Y.Z, newer than VERSION, and not tagged yet; CHANGELOG.md
#    has an "## Unreleased" section with entries; `hecatron self-test` passes.
# 2. On a release-X.Y.Z branch: VERSION, the Unreleased heading becomes
#    "## X.Y.Z (date)" under a new empty Unreleased, and the README's
#    "version X.Y" status line.
# 3. A pull request, merged (squash) once its CI passes.
# 4. The tag vX.Y.Z on the merge commit, and a GitHub release whose notes are
#    the changelog section.
#
# HECATRON_RELEASE_TRAILERS, if set, is appended to the commit message (for
# Co-Authored-By lines and the like). Stops at the first failure; rerunning
# after a failure picks up where it can (an existing branch or PR is reused).
set -euo pipefail

_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${_root}"

_version="${1:-}"
_dry_run=0
[ "${2:-}" = "--dry-run" ] && _dry_run=1
if ! [[ "${_version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "usage: $0 X.Y.Z [--dry-run]" >&2
    exit 2
fi
_tag="v${_version}"
_branch="release-${_version}"
_date="$(date -u +%Y-%m-%d)"

say() { printf '==> %s\n' "$*"; }
fail() { printf 'release: %s\n' "$*" >&2; exit 1; }

# Whether X.Y.Z $1 is newer than $2.
newer() { [ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)" = "$1" ]; }

# The changelog's Unreleased entries (between its heading and the next one).
unreleased_entries() { awk '/^## Unreleased$/ {on = 1; next} /^## / {on = 0} on' CHANGELOG.md; }
# A released section's text, for the release notes.
release_notes() {
    awk -v heading="## ${_version} (" 'index($0, heading) == 1 {on = 1; next} /^## / {on = 0} on' CHANGELOG.md |
        sed -e '/./,$!d'
}

check() {
    say "Checking..."
    command -v gh >/dev/null || fail "gh (GitHub's CLI) is needed"
    gh auth status >/dev/null 2>&1 || fail "gh isn't signed in (gh auth login)"
    git fetch --quiet --tags origin
    [ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || [ "$(git rev-parse --abbrev-ref HEAD)" = "${_branch}" ] ||
        fail "run from main"
    [ -z "$(git status --porcelain)" ] || fail "the working tree has changes"
    if [ "$(git rev-parse --abbrev-ref HEAD)" = "main" ]; then
        [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || fail "main isn't the same as origin/main (git pull)"
    fi
    ! git rev-parse -q --verify "refs/tags/${_tag}" >/dev/null || fail "${_tag} already exists"
    # Rerun on the release branch: the bump is already made, so compare with main.
    _released="$(git show origin/main:VERSION)"
    newer "${_version}" "${_released}" || fail "${_version} isn't newer than ${_released}"
    if [ "$(git rev-parse --abbrev-ref HEAD)" = "main" ]; then
        grep -qx '## Unreleased' CHANGELOG.md || fail "CHANGELOG.md has no '## Unreleased' section"
        [ -n "$(unreleased_entries | tr -d '[:space:]')" ] || fail "CHANGELOG.md's Unreleased section is empty"
    fi
    bin/hecatron self-test >/dev/null 2>&1 || fail "hecatron self-test fails; run it to see why"
}

bump() {
    echo "${_version}" > VERSION
    python3 - "${_version}" "${_date}" <<'EOF'
import re, sys
version, date = sys.argv[1], sys.argv[2]
with open("CHANGELOG.md") as f:
    text = f.read()
text = text.replace("## Unreleased\n", f"## Unreleased\n\n## {version} ({date})\n", 1)
with open("CHANGELOG.md", "w") as f:
    f.write(text)
major_minor = ".".join(version.split(".")[:2])
with open("README.md") as f:
    text = f.read()
text = re.sub(r"(Hecatron is young: version )\d+\.\d+\.", rf"\g<1>{major_minor}.", text)
with open("README.md", "w") as f:
    f.write(text)
EOF
}

check
if [ "${_dry_run}" -eq 1 ]; then
    bump
    say "Would release ${_tag} with these changes:"
    git --no-pager diff --stat
    git --no-pager diff -- VERSION README.md
    say "Release notes:"
    release_notes
    git checkout --quiet -- VERSION CHANGELOG.md README.md
    exit 0
fi

if [ "$(git rev-parse --abbrev-ref HEAD)" = "main" ]; then
    git checkout --quiet -B "${_branch}"
    bump
    _message="Release ${_version}"$'\n\n'"$(release_notes)"
    [ -n "${HECATRON_RELEASE_TRAILERS:-}" ] && _message="${_message}"$'\n\n'"${HECATRON_RELEASE_TRAILERS}"
    git commit --quiet -am "${_message}"
fi
say "Pushing ${_branch} and opening the pull request..."
git push --quiet --force-with-lease -u origin "${_branch}"
_pr="$(gh pr view "${_branch}" --json number,state --jq 'select(.state == "OPEN") | .number' 2>/dev/null || true)"
if [ -z "${_pr}" ]; then
    gh pr create --base main --head "${_branch}" --title "Release ${_version}" --body "$(release_notes)" >/dev/null
    _pr="$(gh pr view "${_branch}" --json number --jq .number)"
fi
say "Waiting for #${_pr}'s checks..."
sleep 10
gh pr checks "${_pr}" --watch --fail-fast >/dev/null || fail "#${_pr}'s checks failed; fix them and rerun"
say "Merging #${_pr}..."
gh pr merge "${_pr}" --squash --delete-branch >/dev/null
_commit="$(gh pr view "${_pr}" --json mergeCommit --jq .mergeCommit.oid)"
git checkout --quiet main
git pull --quiet --ff-only origin main
say "Tagging ${_tag} at ${_commit:0:7} and publishing the release..."
git tag -a "${_tag}" -m "hecatron ${_version}" "${_commit}"
git push --quiet origin "${_tag}"
gh release create "${_tag}" --title "hecatron ${_version}" --notes "$(release_notes)" --verify-tag >/dev/null
say "Released $(gh release view "${_tag}" --json url --jq .url)"
