#!/usr/bin/env bash
# Run a command as a fresh developer on a throwaway Linux container, with this
# checkout mounted read-only at /hecatron and bin/ on PATH.
#
#   tests/docker/run.sh ubuntu:24.04 hecatron doctor go
#   tests/docker/run.sh debian:stable bash   # interactive
set -euo pipefail

if [ "$#" -lt 1 ]; then
    echo "usage: $0 <base-image> [command...]" >&2
    exit 2
fi
_base="$1"
shift

_here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_checkout="$(cd "${_here}/../.." && pwd)"
_image="hecatron-test:$(echo "${_base}" | tr ':/' '--')"

docker build --quiet --build-arg "BASE=${_base}" --tag "${_image}" "${_here}" >/dev/null

_tty=()
[ -t 0 ] && [ -t 1 ] && _tty=(--interactive --tty)
exec docker run --rm ${_tty[@]+"${_tty[@]}"} \
    --volume "${_checkout}:/hecatron:ro" \
    --env PATH="/hecatron/bin:/home/dev/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" \
    "${_image}" "$@"
