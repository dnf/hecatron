"""Python's default hooks and environment: which tests pytest is asked to
collect (through a fake uv that prints how it was called), and the `python`
link install.sh makes where only python3 is."""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402
import project  # noqa: E402

LINK_PYTHON = os.path.join(HOME, "languages", "python", "link-python.sh")


class PytestTest(Fixture):
    """The test hook, with a fake uv: the pytest arguments it passes."""

    def setUp(self):
        super().setUp()
        write(self.path(".fakes", "uv"), '#!/bin/sh\nprintf "%s\\n" "$@" >"$UV_ARGS"\n', 0o755)

    def files(self, files: dict[str, str]) -> None:
        for name, content in files.items():
            write(self.path(name), content)

    def pytest_args(self) -> list[str]:
        environ = {**os.environ, "PATH": self.path(".fakes") + ":" + os.environ["PATH"],
                   "HECATRON_ROOT": self.root, "HECATRON_LANG": "python", "UV_ARGS": self.path(".args")}
        environ.pop("HECATRON_TEST_REPORTS", None)
        hook = os.path.join(HOME, "languages", "python", "hooks", "test.sh")
        result = subprocess.run(["bash", hook], cwd=self.root, env=environ, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        with open(self.path(".args")) as args:
            argv = args.read().splitlines()
        return argv[argv.index("pytest", argv.index("--with") + 2) + 1:]

    def test_default_discovery(self):
        self.files({"pyproject.toml": "", "tests/test_thing.py": ""})
        self.assertEqual(self.pytest_args(), [])

    def test_test_files_named_like_classes_are_collected(self):
        # antlr4's Python runtime: tests/TestFileStream.py.
        self.files({"pyproject.toml": "", "tests/TestFileStream.py": ""})
        self.assertEqual(self.pytest_args(), ["-o", "python_files=test_*.py *_test.py Test*.py"])

    def test_not_beside_default_names(self):
        self.files({"pyproject.toml": "", "tests/TestHelpers.py": "", "tests/test_thing.py": ""})
        self.assertEqual(self.pytest_args(), [])

    def test_not_where_the_project_says_which_files(self):
        self.files({"pyproject.toml": '[tool.pytest.ini_options]\npython_files = ["Test*.py"]\n',
                    "tests/TestFileStream.py": ""})
        self.assertEqual(self.pytest_args(), [])


class PythonLinkTest(Fixture):
    """link-python.sh, with a PATH of only the tools it needs and fakes."""

    def setUp(self):
        super().setUp()
        self.tools = self.path(".tools")
        os.makedirs(self.tools)
        for tool in ("mkdir", "ln", "rm"):
            os.symlink(shutil.which(tool), os.path.join(self.tools, tool))
        self.links = self.path("home", ".local", "share", "hecatron", "python", "bin")

    def fake(self, name: str) -> str:
        write(self.path(".fakes", name), "#!/bin/sh\n", 0o755)
        return self.path(".fakes", name)

    def link(self) -> None:
        path = ":".join([self.links, self.path(".fakes"), self.tools])
        subprocess.run([shutil.which("bash"), LINK_PYTHON], check=True,
                       env={"PATH": path, "HOME": self.path("home")})

    def test_python_links_to_python3_where_there_is_none(self):
        python3 = self.fake("python3")
        self.link()
        self.assertEqual(os.readlink(os.path.join(self.links, "python")), python3)
        self.link()
        self.assertEqual(os.readlink(os.path.join(self.links, "python")), python3)

    def test_another_python_removes_the_link(self):
        self.fake("python3")
        self.link()
        self.fake("python")
        self.link()
        self.assertFalse(os.path.lexists(os.path.join(self.links, "python")))

    def test_nothing_without_python3(self):
        self.link()
        self.assertFalse(os.path.exists(self.links))

    def test_other_languages_hooks_have_the_link_on_path(self):
        # rerun's Rust build runs `python`.
        os.makedirs(self.links)
        self.configure(languages=["rust", "python"])
        saved = os.environ.get("HOME")
        os.environ["HOME"] = self.path("home")
        try:
            path = project.load(self.root).language_env("rust")["path"]
        finally:
            os.environ["HOME"] = saved
        self.assertIn(self.links, path)


if __name__ == "__main__":
    unittest.main()
