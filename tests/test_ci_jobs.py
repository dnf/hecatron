"""libexec/maintenance/ci-jobs.sh: which of CI's jobs a pull request's
changed paths select (.github/workflows/ci.yml reads its outputs)."""

import json
import os
import re
import subprocess
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCRIPT = os.path.join(HOME, "libexec", "maintenance", "ci-jobs.sh")
ALL = {"docs", "test_ubuntu", "test_macos", "self_hosted", "go", "bash_3_2",
       "installer_linux", "installer_macos", "install_parity"}
SCRIPTS = {"test_ubuntu", "test_macos", "bash_3_2", "go"}


def outputs(*args, stdin=""):
    result = subprocess.run(["bash", SCRIPT, *args], input=stdin, capture_output=True, text=True, check=True)
    return dict(line.split("=", 1) for line in result.stdout.splitlines())


def selected(*paths):
    out = outputs(stdin="".join(path + "\n" for path in paths))
    return {job for job in ALL if out[job] == "true"}


class CiJobsTest(unittest.TestCase):
    def test_documentation_runs_only_the_docs_job(self):
        for path in ("README.md", "CONTRIBUTING.md", "LICENSE.md", "llms.txt", "docs/design.md",
                     "docs/coverage.md", "changelog.d/ci-select-jobs.md", "changelog.d/README.md",
                     "assets/logo/logo.svg"):
            with self.subTest(path=path):
                self.assertEqual(selected(path), {"docs"})

    def test_nothing_changed_runs_nothing(self):
        self.assertEqual(selected(), set())
        self.assertEqual(outputs()["test_os"], "[]")

    def test_the_go_advisors_tests_read_its_doc(self):
        self.assertEqual(selected("docs/advisor.md"), {"docs", "go"})

    def test_go_code(self):
        for path in ("cmd/hecatron/main.go", "internal/project/project.go", "internal/generate/ci_test.go"):
            with self.subTest(path=path):
                self.assertEqual(selected(path), {"go", "self_hosted"})
        self.assertEqual(selected("go.mod"), {"go", "self_hosted", "install_parity"})
        self.assertEqual(selected("cmd/hecatron/installdata.go"), {"go", "self_hosted", "install_parity"})

    def test_golden_files_the_python_suite_reads(self):
        self.assertEqual(selected("internal/reports/testdata/test.summary.md"),
                         {"go", "self_hosted", "test_ubuntu", "bash_3_2"})

    def test_the_go_installer(self):
        self.assertEqual(selected("internal/install/plan.go"),
                         {"go", "self_hosted", "install_parity", "test_ubuntu", "bash_3_2"})

    def test_the_launcher_runs_everything_but_parity(self):
        self.assertEqual(selected("bin/hecatron"), ALL - {"docs", "install_parity"})

    def test_the_installer(self):
        self.assertEqual(selected("install.sh"),
                         {"test_ubuntu", "self_hosted", "installer_linux", "installer_macos"})
        self.assertEqual(selected("tests/installer/check.sh"), SCRIPTS | {"installer_linux"})

    def test_libexec(self):
        self.assertEqual(selected("libexec/commands/run.sh"), SCRIPTS | {"self_hosted"})
        self.assertEqual(selected("libexec/lib/helpers.sh"), SCRIPTS | {"self_hosted", "install_parity"})
        self.assertEqual(selected("libexec/maintenance/ci-jobs.sh"), SCRIPTS | {"self_hosted"})

    def test_language_definitions(self):
        self.assertEqual(selected("languages/rust/doctor.sh"), SCRIPTS)
        self.assertEqual(selected("languages/rust/config.json"), SCRIPTS)
        self.assertEqual(selected("languages/rust/install.sh"), SCRIPTS | {"install_parity"})
        self.assertEqual(selected("languages/rust/install.json"), SCRIPTS | {"install_parity"})
        self.assertEqual(selected("languages/raku/doctor.sh"), SCRIPTS | {"install_parity"})
        self.assertEqual(selected("languages/_common.sh"), SCRIPTS | {"self_hosted", "install_parity"})
        self.assertEqual(selected("languages/go/config.json"), SCRIPTS | {"self_hosted"})

    def test_parity_languages_match_the_workflow(self):
        with open(os.path.join(HOME, ".github", "workflows", "ci.yml"), encoding="utf-8") as f:
            workflow = re.search(r"tests/docker/parity\.sh ubuntu:24\.04 (.*)", f.read()).group(1).split()
        for lang in workflow:
            with self.subTest(lang=lang):
                self.assertIn("install_parity", selected("languages/%s/doctor.sh" % lang))

    def test_tests(self):
        self.assertEqual(selected("tests/test_hecatron.py"), {"test_ubuntu", "test_macos", "bash_3_2"})
        self.assertEqual(selected("tests/output/fixtures/x/expected"), SCRIPTS)
        self.assertEqual(selected("tests/docker/parity.sh"), SCRIPTS | {"install_parity"})
        self.assertEqual(selected("tests/docker/run.sh"), SCRIPTS | {"install_parity", "installer_linux"})

    def test_the_skill(self):
        self.assertEqual(selected("plugins/hecatron/skills/hecatron/SKILL.md"), {"test_ubuntu", "go"})

    def test_workflows_and_unknown_paths_run_everything(self):
        for path in (".github/workflows/ci.yml", ".github/workflows/coverage.yml", "VERSION",
                     ".hecatron/config.json", "Makefile", "newdir/x.md"):
            with self.subTest(path=path):
                self.assertEqual(selected(path), ALL)

    def test_a_mixed_change_is_the_union(self):
        self.assertEqual(selected("README.md", "cmd/hecatron/main.go"), {"docs", "go", "self_hosted"})

    def test_all(self):
        out = outputs("--all")
        self.assertEqual({job for job in ALL if out[job] == "true"}, ALL)
        self.assertEqual(json.loads(out["test_os"]), ["ubuntu-latest", "macos-latest"])

    def test_test_matrix(self):
        self.assertEqual(json.loads(outputs(stdin="install.sh\n")["test_os"]), ["ubuntu-latest"])
        self.assertEqual(json.loads(outputs(stdin="bin/hct\n")["test_os"]), ["ubuntu-latest", "macos-latest"])

    def test_a_last_line_without_a_newline_counts(self):
        self.assertEqual(outputs(stdin="README.md\ncmd/x.go")["go"], "true")


if __name__ == "__main__":
    unittest.main()
