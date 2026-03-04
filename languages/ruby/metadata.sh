#!/usr/bin/env bash
# Update Ruby package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's|s\.summary     = "[^"]*"|s.summary     = "'"${PT_DESCRIPTION}"'"|' \
    "${ROOT}/src/ruby/parent_time.gemspec"
_sed_i 's|s\.license     = "[^"]*"|s.license     = "'"${PT_LICENSE}"'"|' \
    "${ROOT}/src/ruby/parent_time.gemspec"
_sed_i 's|s\.authors     = \["[^"]*"\]|s.authors     = ["'"${PT_AUTHOR}"'"]|' \
    "${ROOT}/src/ruby/parent_time.gemspec"
_sed_i 's|s\.homepage    = "[^"]*"|s.homepage    = "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/src/ruby/parent_time.gemspec"
_sed_i 's|"source_code_uri" => "[^"]*"|"source_code_uri" => "'"${PT_REPOSITORY}"'/tree/main/src/ruby"|' \
    "${ROOT}/src/ruby/parent_time.gemspec"
_sed_i 's|"bug_tracker_uri" => "[^"]*"|"bug_tracker_uri" => "'"${PT_REPOSITORY}"'/issues"|' \
    "${ROOT}/src/ruby/parent_time.gemspec"
_sed_i 's|"homepage_uri" => "[^"]*"|"homepage_uri" => "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/src/ruby/parent_time.gemspec"
