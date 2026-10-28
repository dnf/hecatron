- **Python tests in monorepos:** `hct test python` in a directory without
  a `pyproject.toml`, `setup.py` or `setup.cfg` now tests the Python
  projects in the directories below (`py-polars/`, `python-package/`),
  each in its own environment, instead of running pytest over the whole
  tree; with none below, pytest runs there on its own as before.
