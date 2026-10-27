#!/usr/bin/env bash
# Default test hook: RSpec when the bundle has it and there's a spec/, else
# the Rakefile's test task.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
ruby_project
bundle_deps
if [ -d spec ] && gem_listed rspec; then
    ruby_run rspec
elif [ -f "${ruby_root}/Rakefile" ] && rake_has test; then
    ruby_run rake test
else
    unknown "no spec/ with RSpec in the bundle, and no Rakefile with a test task"
fi
