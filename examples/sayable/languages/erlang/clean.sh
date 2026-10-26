#!/usr/bin/env bash
# Remove one library's Erlang build outputs.
set -euo pipefail

rm -rf _build rebar3.crashdump
