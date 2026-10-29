#!/usr/bin/env bash
# Check for the Quicklisp the hooks fetch dependencies with (see install.sh).
source "$(dirname "$0")/../_common.sh"

if [ ! -f "$(hecatron_data)/quicklisp/setup.lisp" ]; then
    problem "Quicklisp not installed (the hooks fetch a system's dependencies with it)"
fi
doctor_done
