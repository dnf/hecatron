"""docs/guide/coverage.md, the coverage matrix (tests/coverage/matrix.py)."""

import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "coverage"))
import matrix  # noqa: E402


def result(platform="linux-x86_64-ubuntu24.04", date="2026-10-27", commit="038fda3", **steps):
    record = {"language": "go", "platform": platform, "date": date, "commit": commit, "evidence": "fresh",
              "install": "pass", "doctor": "pass", "rerun": "pass", "smoke": "pass"}
    record.update(steps)
    return record


class MarkTest(unittest.TestCase):
    def test_each_status(self):
        self.assertEqual(matrix.mark(result()), "✅")
        self.assertEqual(matrix.mark(result(doctor="fail")), "❌")
        self.assertEqual(matrix.mark(result(install="unsupported")), "➖")
        self.assertEqual(matrix.mark(result(evidence="in-use")), "💻")
        self.assertEqual(matrix.mark(None), "❔")

    def test_a_result_from_before_the_smoke_test_is_not_checked(self):
        self.assertEqual(matrix.mark(result(smoke="none")), "❔")


class CheckedAtTest(unittest.TestCase):
    def test_newest_run_with_its_commit(self):
        results = {"a": result(date="2026-10-26", commit=""), "b": result(date="2026-10-27", commit="038fda3abc")}
        self.assertEqual(matrix.checked_at(results, "linux-x86_64-ubuntu24.04"), "2026-10-27 at 038fda3")
        self.assertEqual(matrix.checked_at(results, "macos-x86_64"), "never")


class DocTest(unittest.TestCase):
    def test_coverage_md_is_generated_from_the_results(self):
        with open(matrix.DOC, encoding="utf-8") as f:
            self.assertEqual(f.read(), matrix.render_text(matrix.load()),
                             "docs/guide/coverage.md is out of date: python3 tests/coverage/matrix.py render")


if __name__ == "__main__":
    unittest.main()
