#!/usr/bin/env bash
# Default audit hook: pip-audit (see audit_python in languages/_audit.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_audit.sh"
audit_python
