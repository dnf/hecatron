#!/usr/bin/env bash
# Update Ruby package metadata
source "$(dirname "$0")/../_common.sh"

_sed_i 's|s\.summary     = "[^"]*"|s.summary     = "'"${PT_DESCRIPTION}"'"|' \
    "${ROOT}/sayable-age/src/ruby/sayable_age.gemspec"
_sed_i 's|s\.license     = "[^"]*"|s.license     = "'"${PT_LICENSE}"'"|' \
    "${ROOT}/sayable-age/src/ruby/sayable_age.gemspec"
_sed_i 's|s\.authors     = \["[^"]*"\]|s.authors     = ["'"${PT_AUTHOR}"'"]|' \
    "${ROOT}/sayable-age/src/ruby/sayable_age.gemspec"
_sed_i 's|s\.homepage    = "[^"]*"|s.homepage    = "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/sayable-age/src/ruby/sayable_age.gemspec"
_sed_i 's|"source_code_uri" => "[^"]*"|"source_code_uri" => "'"${PT_REPOSITORY}"'/tree/main/src/ruby"|' \
    "${ROOT}/sayable-age/src/ruby/sayable_age.gemspec"
_sed_i 's|"bug_tracker_uri" => "[^"]*"|"bug_tracker_uri" => "'"${PT_REPOSITORY}"'/issues"|' \
    "${ROOT}/sayable-age/src/ruby/sayable_age.gemspec"
_sed_i 's|"homepage_uri" => "[^"]*"|"homepage_uri" => "'"${PT_REPOSITORY}"'"|' \
    "${ROOT}/sayable-age/src/ruby/sayable_age.gemspec"
