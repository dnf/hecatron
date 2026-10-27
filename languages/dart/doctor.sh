#!/usr/bin/env bash
# Check for Flutter where a component is a Flutter package (see install.sh).
source "$(dirname "$0")/../_common.sh"

if flutter_missing; then problem "flutter not installed (a component is a Flutter package)"; fi
doctor_done
