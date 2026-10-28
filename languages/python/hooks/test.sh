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

# Whether a file named like the glob is here or below, outside version
# control, environments, dependencies and build output.
_skipped_dirs=('.*' node_modules vendor third_party third-party external
    testdata test-data test_data fixtures build target out dist bin obj)
has_below() {
    local _skip=() _dir
    for _dir in "${_skipped_dirs[@]}"; do _skip+=(-o -name "${_dir}"); done
    [ -n "$(find . \( -type d ! -path . \( "${_skip[@]:1}" \) -prune \) -o \
        -type f -name "$1" -print -quit 2>/dev/null)" ]
}

_args=()
# Test files named TestThing.py (antlr4's Python runtime), where none has
# the names pytest collects by default, and the project doesn't say which.
if ! pytest_sets python_files && has_below 'Test*.py' && ! has_below 'test_*.py' && ! has_below '*_test.py'; then
    _args+=(-o "python_files=test_*.py *_test.py Test*.py")
fi
if [ -n "${HECATRON_TEST_REPORTS:-}" ]; then _args+=(--junitxml="${HECATRON_TEST_REPORTS}/pytest.xml"); fi
if [ -f pyproject.toml ] || [ -f setup.py ] || [ -f setup.cfg ]; then
    run uv run --all-extras --all-groups --with pytest pytest ${_args[@]+"${_args[@]}"}
else
    run uv run --with pytest pytest ${_args[@]+"${_args[@]}"}
fi
