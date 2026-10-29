#!/usr/bin/env bash
# Check for SWI-Prolog's PPA on Ubuntu, whose SWI-Prolog is too old for some
# packs (see install.sh).
source "$(dirname "$0")/../_common.sh"

if [ "${OS}" = linux ] && apt_source_missing swi-prolog ubuntu; then
    problem "apt source swi-prolog not added ($(apt_source_file swi-prolog))"
fi
doctor_done
