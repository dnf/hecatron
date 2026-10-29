#!/usr/bin/env bash
# Default test hook: pytest, in the project's environment (with every extra
# and dependency group, where test dependencies usually are) through uv, else
# on its own. Without a project here, the projects in the directories below,
# each (a monorepo's py-polars/ or python-package/), else pytest here on its
# own. Asked for a JUnit report (HECATRON_TEST_REPORTS), pytest writes one
# there.
source "$(dirname "$0")/../../_hooks.sh"
has_any pyproject.toml setup.py setup.cfg || run_below_with pyproject.toml setup.py setup.cfg

# Whether the project sets one of pytest's options (python_files,
# testpaths) in a file pytest reads it from.
pytest_sets() {
    local _file
    for _file in pytest.ini .pytest.ini pyproject.toml setup.cfg tox.ini; do
        [ -f "${_file}" ] && grep -qF "$1" "${_file}" && return 0
    done
    return 1
}

# Directories never searched: version control, environments, dependencies,
# test data and build output (dirs_below_with's).
_skipped_dirs=('.*' node_modules vendor third_party third-party external
    testdata test-data test_data fixtures build target out dist bin obj)
# Whether a file named like the glob is here or below.
has_below() {
    local _skip=() _dir
    for _dir in "${_skipped_dirs[@]}"; do _skip+=(-o -name "${_dir}"); done
    [ -n "$(find . \( -type d ! -path . \( "${_skip[@]:1}" \) -prune \) -o \
        -type f -name "$1" -print -quit 2>/dev/null)" ]
}

# The Python projects inside this one, at most three levels down, only the
# outermost where one is inside another (pydantic-core in pydantic,
# ast-grep's crates/pyo3), sorted.
nested_projects() {
    local _skip=() _dir
    for _dir in "${_skipped_dirs[@]}"; do _skip+=(-o -name "${_dir}"); done
    find . -maxdepth 4 \( -type d ! -path . \( "${_skip[@]:1}" \) -prune \) -o \
        -type f \( -name pyproject.toml -o -name setup.py \) -print 2>/dev/null |
        sed 's|/[^/]*$||' | LC_ALL=C sort -u |
        awk '$0 == "." { next }
            { sub(/^\.\//, "")
              for (i = 1; i <= n; i++) if (index($0, kept[i] "/") == 1) next
              kept[++n] = $0; print }'
}

_args=()
# Test files named TestThing.py (antlr4's Python runtime), where none has
# the names pytest collects by default, and the project doesn't say which.
if ! pytest_sets python_files && has_below 'Test*.py' && ! has_below 'test_*.py' && ! has_below '*_test.py'; then
    _args+=(-o "python_files=test_*.py *_test.py Test*.py")
fi
_project=0
if [ -f pyproject.toml ] || [ -f setup.py ] || [ -f setup.cfg ]; then _project=1; fi
# A project inside this one has an environment of its own, which this one's
# doesn't install: its tests aren't this one's, unless testpaths say so.
if [ "${_project}" -eq 1 ] && ! pytest_sets testpaths; then
    while IFS= read -r _dir; do _args+=(--ignore="${_dir}"); done < <(nested_projects)
fi
if [ -n "${HECATRON_TEST_REPORTS:-}" ]; then _args+=(--junitxml="${HECATRON_TEST_REPORTS}/pytest.xml"); fi
if [ "${_project}" -eq 1 ]; then
    run uv run --all-extras --all-groups --with pytest pytest ${_args[@]+"${_args[@]}"}
else
    run uv run --with pytest pytest ${_args[@]+"${_args[@]}"}
fi
