#!/usr/bin/env bash
# Check for the Maven or Gradle the components build with, and the JDKs their
# builds need besides the default (see install.sh).
source "$(dirname "$0")/../_common.sh"

while IFS= read -r _tool; do
    [ -n "${_tool}" ] && problem "${_tool} not installed (a component builds with it and ships no wrapper)"
done < <(jvm_build_tools_missing java)
while IFS= read -r _feature; do
    [ -n "${_feature}" ] && problem "JDK ${_feature} not installed (a component's build needs it)"
done < <(jvm_jdks_missing java)
doctor_done
