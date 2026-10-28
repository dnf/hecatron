"""The languages' uninstall.sh scripts remove only what is their own: never a
toolchain another language's installer also installs (CMake, Node.js, the
.NET SDK, Erlang/OTP, uv, Homebrew's gcc), nothing the operating system
depends on (the distribution's python3), and never `apt-get autoremove`.

Each script runs on macOS and on Linux (uname is a stub) with every command
that could change the machine (sudo, apt-get, brew, npm, uv, rustup, rm...)
replaced by a stub that only logs its arguments. The real uninstallers are
run after their installers in containers (tests/docker)."""

from __future__ import annotations

import os
import shlex
import shutil
import stat
import subprocess
import tempfile
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LANGUAGES = os.path.join(HOME, "languages")

# Logs "<name> <args>" and succeeds; some answer the questions the scripts ask.
STUB = """#!/usr/bin/env bash
echo "$(basename "$0") $*" >>"${STUB_LOG}"
case "$(basename "$0") $1" in
    "id -u") echo 1000 ;;
    "uname -s") echo "${STUB_UNAME}" ;;
    "uname -m") echo x86_64 ;;
    "brew --prefix") echo "${HOME}/homebrew" ;;
    "dpkg-query "*) printf '%s' "${STUB_DPKG:-}" ;;
    "uv tool") printf '%s' "${STUB_UV_TOOLS:-}" ;;
esac
exit 0
"""
STUBS = ("sudo", "apt-get", "brew", "npm", "uv", "rustup", "rm", "id", "uname", "dpkg-query", "opam",
         # present, so the scripts say they left them
         "cmake", "node", "dotnet", "erl", "python3")

# What no uninstaller may remove: by apt, by Homebrew, or as a path.
PROTECTED_APT = {"python3", "python3-venv", "python3-minimal", "cmake", "nodejs", "build-essential", "gcc",
                 "g++", "make", "perl", "bash", "erlang", "erlang-dev", "erlang-tools", "dotnet-sdk-10.0",
                 "php-common", "php8.3-common"}
PROTECTED_BREW = {"cmake", "gcc", "node", "dotnet", "erlang", "uv", "openjdk"}
PROTECTED_PATHS = ("/opt/dotnet", "/opt/node", "/usr/local/bin/node", "/usr/local/bin/npm",
                   "/usr/local/bin/dotnet", "/.local/bin/uv", "/.local/share/hecatron/node",
                   "/.local/share/hecatron/dotnet", "/usr/lib/jvm")


def languages():
    return sorted(name for name in os.listdir(LANGUAGES)
                  if os.path.isfile(os.path.join(LANGUAGES, name, "uninstall.sh")))


class UninstallTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.bin = os.path.join(self.tmp.name, "bin")
        self.home = os.path.join(self.tmp.name, "home")
        os.makedirs(self.bin)
        os.makedirs(self.home)
        for name in STUBS:
            path = os.path.join(self.bin, name)
            with open(path, "w", encoding="utf-8") as f:
                f.write(STUB)
            os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR)

    def uninstall(self, lang: str, os_name: str, **env: str) -> tuple[list[list[str]], str]:
        """The commands an uninstall.sh ran (as argument lists) and its output."""
        log = os.path.join(self.tmp.name, "log")
        open(log, "w").close()
        # bash's own directory too: /usr/local/bin in CI's bash:3.2 image.
        bash_dir = os.path.dirname(shutil.which("bash") or "/bin/bash")
        run_env = {"PATH": f"{self.bin}:{bash_dir}:/usr/bin:/bin", "HOME": self.home, "HECATRON_ROOT": self.tmp.name,
                   "STUB_LOG": log, "STUB_UNAME": {"macos": "Darwin", "linux": "Linux"}[os_name], **env}
        result = subprocess.run(["bash", os.path.join(LANGUAGES, lang, "uninstall.sh")], env=run_env,
                                capture_output=True, text=True, timeout=60)
        self.assertEqual(result.returncode, 0, f"{lang} on {os_name}: {result.stderr}")
        with open(log, encoding="utf-8") as f:
            commands = [shlex.split(line) for line in f if not line.startswith(("id ", "uname "))]
        return commands, result.stdout

    @staticmethod
    def unsudo(command: list[str]) -> list[str]:
        return command[1:] if command[:1] == ["sudo"] else command

    def test_no_uninstaller_removes_a_shared_or_system_package(self):
        for lang in languages():
            for os_name in ("macos", "linux"):
                with self.subTest(lang=lang, os=os_name):
                    commands, _ = self.uninstall(lang, os_name, STUB_DPKG="ii php8.3-common\nii php8.3-cli\n")
                    for command in map(self.unsudo, commands):
                        if command[:1] == ["apt-get"]:
                            self.assertNotIn("autoremove", command)
                            if "remove" in command or "purge" in command:
                                packages = [arg for arg in command[1:] if not arg.startswith("-")]
                                self.assertFalse(PROTECTED_APT & set(packages), command)
                                self.assertFalse([p for p in packages if "*" in p], command)
                        if command[:2] == ["brew", "uninstall"]:
                            self.assertFalse(PROTECTED_BREW & set(command[2:]), command)
                        if command[:1] == ["rm"]:
                            for path in command[1:]:
                                self.assertFalse(path.startswith(self.home + "/.local/bin/uv"), command)
                                self.assertFalse(any(path.rstrip("/").endswith(p) for p in PROTECTED_PATHS),
                                                 command)

    def test_the_shared_toolchains_are_left_with_a_line_saying_why(self):
        left = {
            "c": "Left CMake installed: C, C++, Objective-C, COBOL and Fortran share it.",
            "cpp": "Left CMake installed",
            "fortran": "Left CMake installed",
            "javascript": "Left Node.js installed: TypeScript, JavaScript and Elm share it.",
            "typescript": "Left Node.js installed",
            "csharp": "Left the .NET SDK installed: C#, F# and VB.NET share it.",
            "fsharp": "Left the .NET SDK installed",
            "vbnet": "Left the .NET SDK installed",
            "erlang": "Left Erlang/OTP installed: Elixir and Gleam run on it.",
            "python": "Left uv installed",
        }
        for lang, line in left.items():
            for os_name in ("macos", "linux"):
                with self.subTest(lang=lang, os=os_name):
                    self.assertIn(line, self.uninstall(lang, os_name)[1])

    def test_python_leaves_the_systems_python3_and_removes_uvs_pythons(self):
        commands, output = self.uninstall("python", "linux")
        self.assertIn("Left the system's python3 installed", output)
        self.assertIn(["uv", "python", "uninstall", "--all"], commands)
        self.assertNotIn("apt-get", [self.unsudo(c)[0] for c in commands])
        commands, _ = self.uninstall("python", "macos")
        self.assertIn(["brew", "uninstall", "python"], commands)

    def test_python_leaves_uvs_pythons_while_mojo_runs_on_one(self):
        commands, output = self.uninstall("python", "linux", STUB_UV_TOOLS="mojo v0.26.1.0\n- mojo\n")
        self.assertNotIn(["uv", "python", "uninstall", "--all"], commands)
        self.assertIn("Left uv's Python builds installed: Mojo runs on one.", output)

    def test_fortran_leaves_homebrews_gcc(self):
        commands, output = self.uninstall("fortran", "macos")
        self.assertIn("Left Homebrew's gcc installed", output)
        self.assertNotIn("gcc", [arg for c in commands if c[:2] == ["brew", "uninstall"] for arg in c])
        commands, _ = self.uninstall("fortran", "linux")
        self.assertIn(["sudo", "apt-get", "remove", "-y", "gfortran"], commands)

    def test_php_removes_only_the_packages_its_installer_installs(self):
        installed = ("ii php-common\nii php8.3-common\nii php8.3-cli\nii php8.3-xml\nii php8.4-mbstring\n"
                     "ii php-cli\nii php8.3-opcache\nii libapache2-mod-php8.3\nrc php8.2-cli\n")
        commands, _ = self.uninstall("php", "linux", STUB_DPKG=installed)
        removes = [self.unsudo(c) for c in commands if self.unsudo(c)[:2] == ["apt-get", "remove"]]
        self.assertEqual(removes, [["apt-get", "remove", "-y", "php8.3-cli", "php8.3-xml", "php8.4-mbstring",
                                    "php-cli"]])
        commands, _ = self.uninstall("php", "linux")
        self.assertFalse([c for c in commands if self.unsudo(c)[:1] == ["apt-get"]])

    def test_d_removes_dub_with_ldc(self):
        commands, _ = self.uninstall("d", "linux")
        self.assertIn(["sudo", "apt-get", "remove", "-y", "ldc", "dub"], commands)
        commands, _ = self.uninstall("d", "macos")
        self.assertIn(["brew", "uninstall", "dub"], commands)

    def test_erlang_removes_rebar3_only(self):
        commands, _ = self.uninstall("erlang", "linux")
        self.assertIn(["sudo", "apt-get", "remove", "-y", "rebar3"], commands)
        commands, _ = self.uninstall("erlang", "macos")
        self.assertIn(["brew", "uninstall", "rebar3"], commands)


if __name__ == "__main__":
    unittest.main()
