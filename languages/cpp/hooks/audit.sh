#!/usr/bin/env bash
# Default audit hook: OSV-Scanner (see audit_osv in languages/_audit.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_audit.sh"
audit_osv
