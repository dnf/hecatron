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
    -v /workspace/age/impl/typescript/node_modules
    -v /workspace/age/impl/python/.venv
    -v /workspace/age/impl/rust/target
    -v /workspace/age/impl/cpp/build
    -v /workspace/age/impl/groovy/.gradle
    -v /workspace/age/impl/java/.gradle
    -v /workspace/age/impl/kotlin/.gradle
)

if [ $# -eq 0 ]; then
    docker run "${DOCKER_OPTS[@]}" sayable-age-linux bash -c '
        (cd age/impl/typescript && npm install --no-audit --no-fund) &&
        (cd age/impl/python && uv sync --extra dev --quiet) &&
        just test
    '
else
    docker run "${DOCKER_OPTS[@]}" sayable-age-linux "$@"
fi
