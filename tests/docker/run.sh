#!/usr/bin/env bash
# Run a command as a fresh developer on a throwaway Linux container, with this
# checkout mounted read-only at /hecatron and hecatron on PATH.
#
#   tests/docker/run.sh ubuntu:24.04 hecatron doctor go
#   tests/docker/run.sh debian:stable bash   # interactive
#
# With HECATRON_TEST_BINARY naming a Linux build of hecatron's binary, that
# binary is hecatron and hct (in /usr/local/sbin, ahead of the checkout's
# bin/hecatron on every PATH, login shells' included). HECATRON_INSTALLER and
# HECATRON_CHECK_SEED are passed on (tests/docker/parity.sh).
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
_extra=()
if [ -n "${HECATRON_TEST_BINARY:-}" ]; then
    _extra+=(--volume "${HECATRON_TEST_BINARY}:/usr/local/sbin/hecatron:ro"
        --volume "${HECATRON_TEST_BINARY}:/usr/local/sbin/hct:ro"
        --env HECATRON_HOME=/hecatron)
fi
[ -n "${HECATRON_INSTALLER:-}" ] && _extra+=(--env "HECATRON_INSTALLER=${HECATRON_INSTALLER}")
[ -n "${HECATRON_CHECK_SEED:-}" ] && _extra+=(--env "HECATRON_CHECK_SEED=${HECATRON_CHECK_SEED}")
# A developer machine's UTF-8 locale: the image's default C locale makes an
# embedded Python's filesystem encoding ASCII, which no newcomer's machine has.
exec docker run --rm ${_tty[@]+"${_tty[@]}"} ${_extra[@]+"${_extra[@]}"} \
    --env LANG=C.UTF-8 \
    --volume "${_checkout}:/hecatron:ro" \
    "${_image}" "$@"
