#!/usr/bin/env bash
# Default lint hook: V's formatter check and v vet.
source "$(dirname "$0")/../../_hooks.sh"
[ -f v.mod ] || unknown "no v.mod"
checks_format
run v fmt -verify .
run v vet .
