#!/usr/bin/env bash
# Run one library's Java (Gradle) test suite.
set -euo pipefail

./gradlew test
