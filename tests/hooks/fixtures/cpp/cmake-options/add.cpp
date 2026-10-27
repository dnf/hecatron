#include "add.hpp"

#include <zlib.h>

// Links zlib, so the build needs the library the installer provides.
int add(int a, int b) { return zlibVersion()[0] ? a + b : 0; }
