#!/usr/bin/env bash
# Default audit hook: bundler-audit (see audit_ruby in languages/_audit.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_audit.sh"
audit_ruby
