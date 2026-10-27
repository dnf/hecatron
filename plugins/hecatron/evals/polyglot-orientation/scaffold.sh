#!/usr/bin/env bash
# A git repository with the same small library in Go and in Python.
set -euo pipefail
mkdir -p go python/src/calc python/tests
printf 'module example.com/calc\n\ngo 1.22\n' > go/go.mod
printf 'package calc\n\nfunc Add(a, b int) int { return a + b }\n' > go/add.go
printf 'package calc\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal("Add(2, 3) != 5")\n\t}\n}\n' > go/add_test.go
printf '[project]\nname = "calc"\nversion = "0.1.0"\nrequires-python = ">=3.9"\n\n[build-system]\nrequires = ["hatchling"]\nbuild-backend = "hatchling.build"\n' > python/pyproject.toml
printf 'def add(a, b):\n    return a + b\n' > python/src/calc/__init__.py
printf 'from calc import add\n\n\ndef test_add():\n    assert add(2, 3) == 5\n' > python/tests/test_add.py
git init -q
git add -A
git -c user.email=eval@example.com -c user.name=eval commit -qm "Add calc in Go and Python"
