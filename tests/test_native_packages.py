"""The libraries the native installers install for a component's build
(cmake_dependencies in languages/_common.sh): what a CMake build looks for,
and, for a build without CMake (a Makefile), the common libraries its
sources include. Package managers are faked."""

from __future__ import annotations

import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402

COMMON = os.path.join(HOME, "languages", "_common.sh")
FAKES = ('OS=linux\napt_install() { echo "apt_install $*"; }\napt_has() { return 0; }\n'
         'brew_install() { echo "brew_install $*"; }\n')


class NativeIncludesTest(Fixture):
    def shell(self, script: str, lang: str = "cpp", impl_path: str = "{lang}") -> str:
        self.configure(languages=[lang], implPath=impl_path)
        env = {**os.environ, "HECATRON_HOME": HOME, "HECATRON_ROOT": self.root}
        result = subprocess.run(["bash", "-c", f"source {COMMON}\n{FAKES}{script}"],
                                cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout.strip()

    def test_a_makefile_builds_includes_name_their_packages(self):
        # CobolCraft's C++ includes <zlib.h>; its Makefile links -lz.
        write(self.path("Makefile"), "all:\n\tg++ -lz cpp/*.cpp\n")
        write(self.path("cpp", "compress.cpp"), '#include <zlib.h>\n#include <cstdio>\n#include "own.h"\n')
        write(self.path("cpp", "net.h"), "  #  include <openssl/ssl.h>\n// #include <curl/curl.h> is a comment\n")
        write(self.path("cpp", "third_party", "x.cpp"), "#include <png.h>\n")
        self.assertEqual(self.shell("native_include_packages cpp"), "OpenSSL\nZLIB")
        self.assertEqual(self.shell("cmake_dependencies cpp").splitlines()[-1], "apt_install libssl-dev zlib1g-dev")

    def test_a_header_the_project_has_itself_doesnt_count(self):
        write(self.path("cpp", "main.cpp"), "#include <zlib.h>\n#include <lz4.h>\n")
        write(self.path("cpp", "zlib", "zlib.h"), "")
        self.assertEqual(self.shell("native_include_packages cpp"), "lz4")

    def test_a_cmake_build_is_read_from_its_cmake_files(self):
        write(self.path("CMakeLists.txt"), "project(x)\nadd_subdirectory(cpp)\n")
        write(self.path("cpp", "main.cpp"), "#include <zlib.h>\n")
        self.assertEqual(self.shell("native_include_packages cpp"), "")

    def test_macos_has_zlib(self):
        write(self.path("c", "main.c"), "#include <zlib.h>\n#include <png.h>\n")
        self.assertEqual(self.shell("OS=macos\ncmake_dependencies c", lang="c").splitlines()[-1],
                         "brew_install libpng")


if __name__ == "__main__":
    unittest.main()
