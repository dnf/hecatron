#!/usr/bin/env bash
# Default docs hook: DocC through swift-docc-plugin, where the package depends on it
# (see docs_swift in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_swift
