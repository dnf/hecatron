#!/usr/bin/env bash
# Run the library's tclsh test script.
set -euo pipefail
tclsh "test_format_$1.tcl"
