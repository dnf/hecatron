#!/usr/bin/env bash
# Check that elm-test matches the Elm compiler (an older elm-test can't build
# with a newer elm), and for elm-review where a component configures it.
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
if elm_review_missing; then
    problem "elm-review not installed (a component configures it in review/elm.json)"
fi
doctor_done
