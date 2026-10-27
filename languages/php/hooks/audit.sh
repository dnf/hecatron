#!/usr/bin/env bash
# Default audit hook: composer audit (see audit_php in languages/_audit.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_audit.sh"
audit_php
