#!/usr/bin/env bash
# Lint one Ruby library ($1: age, date, duration or time).
set -euo pipefail
bundle install >/dev/null
bundle exec rubocop
