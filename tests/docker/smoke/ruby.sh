# Smoke test for ruby: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf "source 'https://rubygems.org'\n" > Gemfile
bundle install --quiet
ruby -e 'puts "hello"'
