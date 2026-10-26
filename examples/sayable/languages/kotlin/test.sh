#!/usr/bin/env bash
# Run one library's Kotlin (Gradle) test suite.
set -euo pipefail

./gradlew test
