#!/usr/bin/env bash
# Check for the Maven or Gradle the components build with (see install.sh).
source "$(dirname "$0")/../_common.sh"

while IFS= read -r _tool; do
    [ -n "${_tool}" ] && problem "${_tool} not installed (a component builds with it and ships no wrapper)"
done < <(jvm_build_tools_missing kotlin)
doctor_done
