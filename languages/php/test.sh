#!/usr/bin/env bash
# Install dependencies and run the PHPUnit suite for one library.
set -euo pipefail

composer install --no-interaction --prefer-dist
composer test
