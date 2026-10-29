#!/usr/bin/env bash
# Default docs hook: EDoc through rebar3, into doc/
# (see docs_erlang in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_erlang
