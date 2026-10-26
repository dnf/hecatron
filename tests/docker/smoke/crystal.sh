# Smoke test for crystal: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'require "json"\nputs({"greeting" => "hello"}.to_json)\n' > hello.cr
crystal build hello.cr && ./hello
shards --version
