#!/usr/bin/env bash
# Analyse and style-check one PHP library ($1: age, date, duration or time).
set -euo pipefail
composer install --quiet
composer analyse
composer cs-check
