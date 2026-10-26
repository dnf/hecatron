#!/usr/bin/env bash
# Run the Octave test script for one library.
set -euo pipefail

octave --no-gui --path . "test_format_${1}.m"
