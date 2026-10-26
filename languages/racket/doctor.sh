#!/usr/bin/env bash
# Check for `raco test` and rackunit (absent from a minimal Racket install).
source "$(dirname "$0")/../_common.sh"

raco test --help >/dev/null 2>&1 || problem "raco test is missing (package compiler-lib)"
racket -l rackunit -e '' >/dev/null 2>&1 || problem "rackunit is missing (package rackunit-lib)"
doctor_done
