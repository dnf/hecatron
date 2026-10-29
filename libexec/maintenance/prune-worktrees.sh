#!/usr/bin/env bash
# Remove the repository's worktrees whose work is in main:
#
#   libexec/maintenance/prune-worktrees.sh --dry-run   # what it would remove
#   libexec/maintenance/prune-worktrees.sh
#
# A worktree goes when it has no uncommitted or untracked changes and its
# commit is in origin/main, or its branch's pull request was merged (a
# squash merge leaves the commit out of main; needs gh). The branches stay.
# Everything else is listed with why it stays: changes, not merged, locked
# (an agent's, still running), or the main worktree.
set -euo pipefail

_dry_run=false
case "${1:-}" in
    "") ;;
    --dry-run) _dry_run=true ;;
    *) echo "usage: $0 [--dry-run]" >&2; exit 2 ;;
esac

git fetch --quiet --prune origin
_main="$(git worktree list --porcelain | sed -n '1s/^worktree //p')"
_has_gh=false
command -v gh >/dev/null 2>&1 && _has_gh=true

# Why a worktree stays, or nothing when it can go.
_keep_reason() {
    local path="$1" branch="$2" locked="$3"
    if [ "${path}" = "${_main}" ]; then echo "the main worktree"; return; fi
    if [ -n "${locked}" ]; then echo "locked"; return; fi
    if [ ! -d "${path}" ]; then return; fi
    if [ -n "$(git -C "${path}" status --porcelain 2>/dev/null)" ]; then echo "has changes"; return; fi
    if git merge-base --is-ancestor "$(git -C "${path}" rev-parse HEAD)" origin/main 2>/dev/null; then return; fi
    if [ -n "${branch}" ] && "${_has_gh}" &&
        [ -n "$(gh pr list --state merged --head "${branch}" --json number --jq '.[0].number' 2>/dev/null)" ]; then
        return
    fi
    echo "not merged"
}

_removed=0
_kept=0
_consider() {
    local path="$1" branch="$2" locked="$3" reason
    [ -n "${path}" ] || return 0
    reason="$(_keep_reason "${path}" "${branch}" "${locked}")"
    if [ -n "${reason}" ]; then
        printf 'keep    %s (%s%s)\n' "${path}" "${branch:-detached}" ", ${reason}"
        _kept=$((_kept + 1))
    elif "${_dry_run}"; then
        printf 'remove  %s (%s)\n' "${path}" "${branch:-detached}"
        _removed=$((_removed + 1))
    elif git worktree remove "${path}"; then
        printf 'removed %s (%s)\n' "${path}" "${branch:-detached}"
        _removed=$((_removed + 1))
    else
        _kept=$((_kept + 1))
    fi
}

_path=""
_branch=""
_locked=""
while IFS= read -r _line; do
    case "${_line}" in
        "worktree "*) _path="${_line#worktree }" ;;
        "branch refs/heads/"*) _branch="${_line#branch refs/heads/}" ;;
        locked*) _locked=yes ;;
        "")
            _consider "${_path}" "${_branch}" "${_locked}"
            _path="" _branch="" _locked=""
            ;;
    esac
done < <(git worktree list --porcelain; echo)

"${_dry_run}" || git worktree prune
if "${_dry_run}"; then
    echo "${_removed} to remove, ${_kept} kept"
else
    echo "${_removed} removed, ${_kept} kept"
fi
