#!/usr/bin/env bash
# Remove one library's C# build outputs (bin and obj of the project and its tests).
set -euo pipefail

case "$1" in
    age) rm -rf SayableAge/bin SayableAge/obj SayableAge.Tests/bin SayableAge.Tests/obj ;;
    date) rm -rf SayableDate/bin SayableDate/obj SayableDate.Tests/bin SayableDate.Tests/obj ;;
    duration) rm -rf SayableDuration/bin SayableDuration/obj SayableDuration.Tests/bin SayableDuration.Tests/obj ;;
    time) rm -rf SayableTime/bin SayableTime/obj SayableTime.Tests/bin SayableTime.Tests/obj ;;
    *) echo "unknown library: $1" >&2; exit 1 ;;
esac
