#!/usr/bin/env bash
# Default build hook: ExtUtils::MakeMaker's or Module::Build's build.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f Makefile.PL ]; then
    makemaker_configure
    run make
elif [ -f Build.PL ]; then
    module_build_configure
    run ./Build
else
    skip "no Makefile.PL or Build.PL to build"
fi
