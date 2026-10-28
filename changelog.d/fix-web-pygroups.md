- **Python test dependencies in dependency groups:** `hct test python`
  and `hct deps python` now install every PEP 735 dependency group
  (`uv run --all-groups`, `uv sync --all-groups`), not only `dev`, so
  test dependencies kept in a `test` or `tests` group are there.
