# Smoke test for cpp: build a hello world with CMake and run it in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
cat > CMakeLists.txt <<'EOF'
cmake_minimum_required(VERSION 3.16)
project(hello CXX)
set(CMAKE_CXX_STANDARD 17)
add_executable(hello hello.cpp)
target_compile_options(hello PRIVATE -Wall -Wextra -Werror)
EOF
cat > hello.cpp <<'EOF'
#include <iostream>
#include <numeric>
#include <vector>

int main() {
    std::vector<int> v{1, 2, 3, 4, 5, 6, 7, 8, 9};
    std::cout << std::accumulate(v.begin(), v.end(), 0) << '\n';
}
EOF
cmake -S . -B build >/dev/null && cmake --build build >/dev/null
_out="$(./build/hello)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"
