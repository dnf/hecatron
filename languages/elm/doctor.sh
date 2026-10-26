#!/usr/bin/env bash
# Check that elm-test matches the Elm compiler (an older elm-test can't build
# with a newer elm).
source "$(dirname "$0")/../_common.sh"

if ! has elm-test; then
    problem "elm-test not found"
elif has elm; then
    _elm="$(elm --version 2>/dev/null)"
    _elm_test="$(elm-test --version 2>/dev/null)"
    case "${_elm_test}" in
        "${_elm}"*) ;;
        *) problem "elm-test ${_elm_test} doesn't match elm ${_elm}" ;;
    esac
fi
doctor_done
