#!/usr/bin/env bash
# Default docs hook: ExDoc (mix docs), where it's a dependency, into doc/
# (see docs_elixir in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_elixir
