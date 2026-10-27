#!/usr/bin/env bash
# Default format hook: swift-format (see format_swift in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_swift
