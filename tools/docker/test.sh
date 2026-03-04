#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../../.."

docker build -t parent-time-linux .

# Anonymous volumes shadow host directories that contain platform-specific
# artifacts (native binaries, build caches). This avoids macOS/Linux
# conflicts without modifying the host filesystem.
DOCKER_OPTS=(
    --rm
    -v "$(pwd)":/workspace
    -v /workspace/src/typescript/node_modules
    -v /workspace/src/python/.venv
    -v /workspace/src/rust/target
    -v /workspace/src/cpp/build
    -v /workspace/src/groovy/.gradle
    -v /workspace/src/java/.gradle
    -v /workspace/src/kotlin/.gradle
)

if [ $# -eq 0 ]; then
    docker run "${DOCKER_OPTS[@]}" parent-time-linux bash -c '
        (cd src/typescript && npm install --no-audit --no-fund) &&
        (cd src/python && uv sync --extra dev --quiet) &&
        just test
    '
else
    docker run "${DOCKER_OPTS[@]}" parent-time-linux "$@"
fi
