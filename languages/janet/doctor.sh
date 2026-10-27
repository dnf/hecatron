#!/usr/bin/env bash
# Check for the jpm a component's project.janet needs (see install.sh).
source "$(dirname "$0")/../_common.sh"

if impl_has janet -name project.janet && ! has jpm; then
    problem "jpm not installed (a component has a project.janet)"
fi
doctor_done
