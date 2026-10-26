# Smoke test for go: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
mkdir hello && cd hello && go mod init hello >/dev/null 2>&1
printf 'package main\n\nimport "fmt"\n\nfunc main() { fmt.Println("hello") }\n' > main.go
go run . && go vet ./... && golangci-lint run ./...
