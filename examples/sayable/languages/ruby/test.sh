#!/usr/bin/env bash
# Install gems and run the library's Minitest suite.
set -euo pipefail
bundle install
bundle exec ruby -Ilib test/test_formatter.rb
