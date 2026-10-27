#!/usr/bin/env bash
# Default clean hook: ExtUtils::MakeMaker's or Module::Build's realclean,
# which also removes the generated Makefile or Build script.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f Makefile ]; then
    run make realclean
    run rm -f Makefile.old
elif [ -x Build ]; then
    run ./Build realclean
elif [ -f Makefile.PL ] || [ -f Build.PL ]; then
    skip "not configured, so nothing is built"
else
    skip "no Makefile.PL or Build.PL, so nothing is built"
fi
