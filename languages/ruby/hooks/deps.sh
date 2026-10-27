#!/usr/bin/env bash
# Default deps hook: bundle install, frozen to Gemfile.lock (see deps_ruby in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_ruby
