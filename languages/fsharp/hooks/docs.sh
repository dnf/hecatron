#!/usr/bin/env bash
# Default docs hook: fsdocs, where it's one of the project's local tools
# (see docs_dotnet in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_dotnet
