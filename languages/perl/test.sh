#!/usr/bin/env bash
# Run the Perl prove suite for one library.
set -euo pipefail

prove -Ilib t/
