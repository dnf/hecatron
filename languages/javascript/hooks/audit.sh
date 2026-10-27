#!/usr/bin/env bash
# Default audit hook: the package manager's audit (npm, pnpm, Yarn or Bun) (see audit_node in languages/_audit.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_audit.sh"
audit_node
