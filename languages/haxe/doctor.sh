#!/usr/bin/env bash
# Check that haxelib starts (it needs the Neko VM) and has a repository to
# install libraries into.
source "$(dirname "$0")/../_common.sh"

if ! has haxelib; then
    problem "haxelib not found"
elif ! haxelib version >/dev/null 2>&1; then
    problem "haxelib fails to start (is Neko installed?)"
elif ! haxelib config >/dev/null 2>&1; then
    problem "haxelib has no repository (haxelib setup)"
fi
doctor_done
