# Smoke test for ruby: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
# racc has a native extension, so this also checks gems can compile.
printf "source 'https://rubygems.org'\ngem 'racc'\n" > Gemfile
bundle config set --local path vendor/bundle
bundle install --quiet
bundle exec ruby -e 'require "racc/cparse"; puts "hello"'
