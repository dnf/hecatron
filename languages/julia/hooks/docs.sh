#!/usr/bin/env bash
# Default docs hook: Documenter's docs/make.jl, into docs/build
# (see docs_julia in languages/_docs.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_docs.sh"
docs_julia
