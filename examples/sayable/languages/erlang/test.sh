#!/usr/bin/env bash
# Compile and run one library's EUnit suite.
set -euo pipefail

rebar3 compile
rebar3 eunit
