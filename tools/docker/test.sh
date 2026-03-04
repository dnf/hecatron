#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../../.."

docker build -t sayable-age-linux .

# Anonymous volumes shadow host directories that contain platform-specific
# artifacts (native binaries, build caches). This avoids macOS/Linux
# conflicts without modifying the host filesystem.
DOCKER_OPTS=(
    --rm
    -v "$(pwd)":/workspace
    -v /workspace/sayable-age/src/typescript/node_modules
    -v /workspace/sayable-age/src/python/.venv
    -v /workspace/sayable-age/src/rust/target
    -v /workspace/sayable-age/src/cpp/build
    -v /workspace/sayable-age/src/groovy/.gradle
    -v /workspace/sayable-age/src/java/.gradle
    -v /workspace/sayable-age/src/kotlin/.gradle
)

if [ $# -eq 0 ]; then
    docker run "${DOCKER_OPTS[@]}" sayable-age-linux bash -c '
        (cd sayable-age/src/typescript && npm install --no-audit --no-fund) &&
        (cd sayable-age/src/python && uv sync --extra dev --quiet) &&
        just test
    '
else
    docker run "${DOCKER_OPTS[@]}" sayable-age-linux "$@"
fi
