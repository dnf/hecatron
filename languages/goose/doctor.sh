#!/usr/bin/env bash
# Check for the C compiler and headers goose compiles programs with (see
# install.sh).
source "$(dirname "$0")/../_common.sh"

if [ "${OS}" = "linux" ]; then
    has cc || problem "no C compiler (cc) for goose to build with"
    [ -f /usr/include/stdint.h ] || problem "no C library headers (/usr/include/stdint.h) for goose to run programs with"
fi
doctor_done
