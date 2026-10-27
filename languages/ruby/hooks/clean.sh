#!/usr/bin/env bash
# Default clean hook: the gems `gem build` and `rake build` leave (./*.gem,
# pkg/), and test coverage reports.
source "$(dirname "$0")/../../_hooks.sh"
run rm -rf ./*.gem pkg coverage .rspec_status
