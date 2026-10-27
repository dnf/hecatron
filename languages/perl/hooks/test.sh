#!/usr/bin/env bash
# Default test hook: ExtUtils::MakeMaker's or Module::Build's test target,
# else prove over t/ (with Carton's local/ when it's there).
source "$(dirname "$0")/../../_hooks.sh"
if [ -f Makefile.PL ]; then
    makemaker_configure
    run make test
elif [ -f Build.PL ]; then
    module_build_configure
    run ./Build test
elif [ -d t ] && [ -d local/lib/perl5 ]; then
    run prove -lr -Ilocal/lib/perl5 t
elif [ -d t ]; then
    run prove -lr t
else
    unknown "no Makefile.PL, Build.PL or t/"
fi
