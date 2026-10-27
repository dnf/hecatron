cmake_minimum_required(VERSION 3.16)
project(add CXX)
set(CMAKE_CXX_STANDARD 17)

# A library found through pkg-config, which the installer installs.
find_package(PkgConfig REQUIRED)
pkg_check_modules(ZLIB REQUIRED IMPORTED_TARGET zlib)

# Tests only when asked, as many projects do; and a second test option that
# needs setup the hooks can't give, which they must leave off.
option(ADD_BUILD_TESTS "Build the tests" OFF)
option(ADD_BUILD_SERVER_TESTS "Build the tests that need a server" OFF)
if(ADD_BUILD_SERVER_TESTS AND NOT ADD_SERVER)
  message(FATAL_ERROR "ADD_BUILD_SERVER_TESTS needs ADD_SERVER")
endif()

add_library(add add.cpp)
target_link_libraries(add PkgConfig::ZLIB)
if(ADD_BUILD_TESTS)
  enable_testing()
  add_executable(add_test add_test.cpp)
  target_link_libraries(add_test add)
  add_test(NAME add_test COMMAND add_test)
endif()
