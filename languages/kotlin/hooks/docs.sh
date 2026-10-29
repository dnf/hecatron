#!/usr/bin/env bash
# Default docs hook: Dokka, where the build applies it, through the build (see jvm_docs_* in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
jvm_hook docs
