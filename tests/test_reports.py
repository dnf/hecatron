"""The SARIF the Go core writes (hct lint and hct audit, through
HECATRON_REPORT_DIR or --output sarif) matches the SARIF 2.1.0 schema,
vendored in tests/schemas/sarif-2.1.0.json from
https://json.schemastore.org/sarif-2.1.0.json (SchemaStore, Apache-2.0; the
OASIS schema it copies); no network. The files checked are
internal/reports' golden files, which its Go tests keep equal to what is
written. GitLab's Code Quality JSON has no published schema: its fields are
checked here as GitLab's documentation lists them. Skipped where jsonschema
isn't installed."""

import glob
import json
import os
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GOLDEN = os.path.join(ROOT, "internal", "reports", "testdata")
SCHEMAS = os.path.join(ROOT, "tests", "schemas")

try:
    import jsonschema
except ImportError:  # pragma: no cover - depends on the machine
    jsonschema = None


def load(path: str):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


@unittest.skipIf(jsonschema is None, "jsonschema is not installed")
class SarifTest(unittest.TestCase):
    def validator(self):
        return jsonschema.Draft7Validator(load(os.path.join(SCHEMAS, "sarif-2.1.0.json")))

    def test_golden_sarif_matches_the_schema(self):
        files = sorted(glob.glob(os.path.join(GOLDEN, "*.sarif")))
        self.assertGreaterEqual(len(files), 2)
        validator = self.validator()
        for path in files:
            with self.subTest(os.path.basename(path)):
                errors = [e.message for e in validator.iter_errors(load(path))]
                self.assertEqual(errors, [])

    def test_schema_rejects_a_bad_log(self):
        log = load(os.path.join(GOLDEN, "lint.sarif"))
        log["runs"][0]["results"][0]["level"] = "fatal"
        self.assertTrue(list(self.validator().iter_errors(log)))


class CodeQualityTest(unittest.TestCase):
    def test_issues_have_gitlabs_fields(self):
        issues = load(os.path.join(GOLDEN, "lint.codequality.json"))
        self.assertTrue(issues)
        for issue in issues:
            self.assertIsInstance(issue["description"], str)
            self.assertIsInstance(issue["check_name"], str)
            self.assertIsInstance(issue["fingerprint"], str)
            self.assertIn(issue["severity"], ["info", "minor", "major", "critical", "blocker"])
            self.assertIsInstance(issue["location"]["path"], str)
            self.assertIsInstance(issue["location"]["lines"]["begin"], int)
        self.assertEqual(len({i["fingerprint"] for i in issues}), len(issues))


if __name__ == "__main__":
    unittest.main()
