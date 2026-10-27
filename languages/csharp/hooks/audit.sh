#!/usr/bin/env bash
# Default audit hook: dotnet list package --vulnerable (see audit_dotnet in languages/_audit.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_audit.sh"
audit_dotnet
