#!/usr/bin/env bash
# Check formatting of one Crystal library ($1: age, date, duration or time).
set -euo pipefail
crystal tool format --check src spec
