#!/usr/bin/env bash
# A git repository with one Go module whose test expects Add(2, 3) to be 6.
set -euo pipefail
mkdir -p go
printf 'module example.com/calc\n\ngo 1.22\n' > go/go.mod
printf 'package calc\n\n// Add returns the sum of a and b.\nfunc Add(a, b int) int { return a + b }\n' > go/add.go
printf 'package calc\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) {\n\tif got := Add(2, 3); got != 6 {\n\t\tt.Fatalf("Add(2, 3) = %%d, want 6", got)\n\t}\n}\n' > go/add_test.go
git init -q
git add -A
git -c user.email=eval@example.com -c user.name=eval commit -qm "Add calc"
