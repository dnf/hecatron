#!/usr/bin/env bash
# Default test hook: the %!test blocks in every .m file, run with Octave's
# test(): those under inst/ for an Octave package (DESCRIPTION), else all
# here. Fails when any test fails, or when there are none.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
_dir="."
[ -f DESCRIPTION ] && [ -d inst ] && _dir="inst"
_files=""
while IFS= read -r -d '' _file; do
    _files="${_files} '${_file//\'/\'\'}'"
done < <(find "${_dir}" -name '*.m' -type f -print0 | sort -z)
[ -n "${_files}" ] || unknown "no .m files in ${_dir}"
run octave --no-gui --no-window-system --quiet --eval "
addpath(genpath('${_dir}'));
passed = 0; total = 0;
for file = {${_files} }
  [n, m] = test(file{1}, 'quiet', stdout);
  passed += n; total += m;
end
printf('%d of %d tests passed\n', passed, total);
if total == 0, error('no %%!test blocks found'); end
exit(passed < total);
"
