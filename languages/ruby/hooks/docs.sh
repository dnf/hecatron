#!/usr/bin/env bash
# Default docs hook: YARD when the bundle has it, else RDoc, into doc/
# (see docs_ruby in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_ruby
