#!/usr/bin/env bash
# Default audit hook: govulncheck (see audit_go in languages/_audit.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_audit.sh"
audit_go
