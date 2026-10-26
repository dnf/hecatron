# Smoke test for dart: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'void main() { print("hello"); }\n' > hello.dart
dart analyze hello.dart && dart run hello.dart
