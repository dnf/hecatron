# Smoke test for crystal: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'require "json"\nputs({"greeting" => "hello"}.to_json)\n' > hello.cr
crystal build hello.cr && ./hello
shards --version
