#!/usr/bin/env bash
# Default test hook: RSpec when the bundle has it and there's a spec/, else
# the Rakefile's test task. A gem with a native extension (ext/) whose
# Rakefile has rake-compiler's compile task is compiled first: its tests
# load the extension, and its test task doesn't always build it.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
ruby_project
bundle_deps
if [ -d "${ruby_root}/ext" ] && [ -f "${ruby_root}/Rakefile" ] && rake_has compile; then
    ruby_run rake compile
fi
if [ -d spec ] && gem_listed rspec; then
    ruby_run rspec
elif [ -f "${ruby_root}/Rakefile" ] && rake_has test; then
    ruby_run rake test
else
    unknown "no spec/ with RSpec in the bundle, and no Rakefile with a test task"
fi
