"""hecatron audit's report reading (libexec/lib/audit.py): each auditor's
JSON as captured from a real run (tests/audit/reports/), the severity
threshold, accepted risks and their expiry, and end-of-life warnings."""

from __future__ import annotations

import contextlib
import io
import json
import os
import re
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "libexec", "lib"))

import audit  # noqa: E402

REPORTS = os.path.join(HOME, "tests", "audit", "reports")


def report_text(name: str) -> str:
    with open(os.path.join(REPORTS, name), encoding="utf-8") as f:
        return f.read()


def run_report(findings_text: str, fmt: str, config: dict | None = None, today: str = "2026-10-27"):
    out = io.StringIO()
    with contextlib.redirect_stdout(out):
        status = audit.report(fmt, findings_text, "lang", "comp", config or {}, today)
    return status, out.getvalue()


class CapturedReportsTest(unittest.TestCase):
    """Each auditor's report from the fixture projects (tests/audit/fixtures/)."""

    def ids(self, fmt: str, name: str) -> set[str]:
        return {i for f in audit.parse(fmt, report_text(name)) for i in f.ids}

    def test_npm_finds_lodash(self):
        findings = audit.parse("npm", report_text("npm.json"))
        self.assertTrue(findings)
        self.assertTrue(all(f.package == "lodash" for f in findings))
        self.assertIn("GHSA-35jh-r3h4-6jhm", {f.id for f in findings})
        self.assertTrue(all(f.severity for f in findings))

    def test_pip_audit_finds_requests_and_urllib3_unrated(self):
        findings = audit.parse("pip-audit", report_text("pip-audit.json"))
        self.assertLessEqual({"requests", "urllib3"}, {f.package for f in findings})
        self.assertTrue(all(f.severity is None for f in findings))

    def test_govulncheck_reports_only_called_code(self):
        # The report also lists advisories for x/text code the fixture
        # doesn't call (GO-2022-1059...): only language.Parse's counts.
        findings = audit.parse("govulncheck", report_text("govulncheck.json"))
        self.assertEqual([(f.id, f.package) for f in findings], [("GO-2021-0113", "golang.org/x/text")])

    def test_composer_finds_psr7(self):
        findings = audit.parse("composer", report_text("composer.json"))
        self.assertIn("guzzlehttp/psr7", {f.package for f in findings})

    def test_bundle_audit_finds_rack(self):
        findings = audit.parse("bundle-audit", report_text("bundle-audit.txt"))
        self.assertTrue(findings)
        self.assertEqual({f.package for f in findings}, {"rack"})

    def test_dotnet_finds_newtonsoft_json(self):
        findings = audit.parse("dotnet", report_text("dotnet.json"))
        self.assertEqual({f.package for f in findings}, {"Newtonsoft.Json"})
        self.assertEqual({f.severity for f in findings}, {"high"})

    def test_osv_finds_the_rust_and_java_advisories(self):
        self.assertTrue({"RUSTSEC-2020-0071"} & self.ids("osv", "osv-rust.json"))
        self.assertIn("CVE-2021-44228", self.ids("osv", "osv-java.json"))

    def test_osv_rates_log4shell_critical(self):
        findings = audit.parse("osv", report_text("osv-java.json"))
        log4shell = next(f for f in findings if "CVE-2021-44228" in f.ids)
        self.assertEqual(log4shell.severity, "critical")


class FormatsTest(unittest.TestCase):
    """Reports no fixture produces: older and other package managers."""

    def test_pnpm_and_npm_6_advisories(self):
        text = json.dumps({"advisories": {"1": {
            "id": 1, "module_name": "minimist", "severity": "moderate", "title": "Prototype Pollution",
            "github_advisory_id": "GHSA-vh95-rmgr-6w4m", "cves": ["CVE-2020-7598"],
            "findings": [{"version": "0.0.8"}], "url": "https://github.com/advisories/GHSA-vh95-rmgr-6w4m"}}})
        [finding] = audit.parse("pnpm", text)
        self.assertEqual((finding.id, finding.package, finding.version, finding.severity),
                         ("GHSA-vh95-rmgr-6w4m", "minimist", "0.0.8", "moderate"))
        self.assertIn("CVE-2020-7598", finding.ids)

    def test_yarn_classic_lines(self):
        line = {"type": "auditAdvisory", "data": {"advisory": {
            "id": 7, "module_name": "minimist", "severity": "high", "github_advisory_id": "GHSA-xvch-5gv4-984h",
            "cves": [], "findings": [{"version": "1.2.5"}]}}}
        text = json.dumps(line) + "\n" + json.dumps(line) + "\n" + json.dumps({"type": "auditSummary", "data": {}})
        [finding] = audit.parse("yarn-classic", text)
        self.assertEqual(finding.severity, "high")

    def test_yarn_berry_lines(self):
        text = json.dumps({"value": "minimist", "children": {
            "ID": 1096287, "Issue": "Prototype Pollution", "URL": "https://github.com/advisories/GHSA-xvch-5gv4-984h",
            "Severity": "critical", "Tree Versions": ["1.2.5"]}})
        [finding] = audit.parse("yarn-berry", text)
        self.assertEqual((finding.id, finding.severity, finding.version), ("GHSA-xvch-5gv4-984h", "critical", "1.2.5"))

    def test_bun(self):
        text = json.dumps({"minimist": [{"id": 1, "url": "https://github.com/advisories/GHSA-xvch-5gv4-984h",
                                         "title": "Prototype Pollution", "severity": "critical",
                                         "vulnerable_versions": "<1.2.6"}]})
        [finding] = audit.parse("bun", text)
        self.assertEqual(finding.id, "GHSA-xvch-5gv4-984h")

    def test_severities_are_normalized(self):
        self.assertEqual(audit.normalize_severity("MEDIUM"), "moderate")
        self.assertEqual(audit.normalize_severity("9.8"), "critical")
        self.assertEqual(audit.normalize_severity("7.5"), "high")
        self.assertIsNone(audit.normalize_severity("Unknown"))

    def test_an_empty_osv_report_has_no_findings(self):
        self.assertEqual(audit.parse("osv", ""), [])

    def test_npm_findings_get_the_installed_version_from_the_lock_file(self):
        """npm audit names the vulnerable range, not the installed version:
        the lock file it read, in the working directory, gives it (as
        internal/deps' withInstalled does)."""
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            with open(os.path.join(tmp, "package-lock.json"), "w", encoding="utf-8") as f:
                json.dump({"lockfileVersion": 3, "packages": {
                    "": {"name": "x"}, "node_modules/lodash": {"version": "4.17.15"},
                    "node_modules/a/node_modules/lodash": {"version": "4.17.10"}, "node_modules/b": "odd"}}, f)
            cwd = os.getcwd()
            os.chdir(tmp)
            try:
                findings = audit.parse("npm", report_text("npm.json"))
            finally:
                os.chdir(cwd)
        self.assertTrue(findings)
        self.assertEqual({f.version for f in findings}, {"4.17.10, 4.17.15"})
        self.assertIn("  lodash 4.17.10, 4.17.15  GHSA-", audit._line(findings[0]))


def finding_report(*severities: str | None) -> str:
    return json.dumps({"advisories": {str(i): {
        "id": i, "module_name": f"pkg{i}", "severity": severity, "github_advisory_id": f"GHSA-aaaa-bbbb-{i:04d}",
        "cves": [f"CVE-2020-{1000 + i}"], "findings": [{"version": "1.0.0"}]} for i, severity in enumerate(severities)}})


class ThresholdTest(unittest.TestCase):
    def setUp(self) -> None:
        self._saved = os.environ.pop("HECATRON_AUDIT_FAIL_ON", None)

    def tearDown(self) -> None:
        os.environ.pop("HECATRON_AUDIT_FAIL_ON", None)
        if self._saved is not None:
            os.environ["HECATRON_AUDIT_FAIL_ON"] = self._saved

    def test_fails_at_high_by_default(self):
        self.assertEqual(run_report(finding_report("moderate", "low"), "pnpm")[0], 0)
        status, out = run_report(finding_report("high", "low"), "pnpm")
        self.assertEqual(status, 1)
        self.assertIn("comp: lang 2 vulnerabilities (critical 0, high 1, moderate 0, low 1)", out)

    def test_fail_on_moves_the_threshold(self):
        os.environ["HECATRON_AUDIT_FAIL_ON"] = "critical"
        self.assertEqual(run_report(finding_report("high"), "pnpm")[0], 0)
        os.environ["HECATRON_AUDIT_FAIL_ON"] = "low"
        self.assertEqual(run_report(finding_report("low"), "pnpm")[0], 1)

    def test_the_project_sets_a_default_and_the_flag_overrides_it(self):
        config = {"audit": {"failOn": "critical"}}
        self.assertEqual(run_report(finding_report("high"), "pnpm", config)[0], 0)
        os.environ["HECATRON_AUDIT_FAIL_ON"] = "high"
        self.assertEqual(run_report(finding_report("high"), "pnpm", config)[0], 1)

    def test_an_unrated_finding_counts_as_high(self):
        status, out = run_report(finding_report(None), "pnpm")
        self.assertEqual(status, 1)
        self.assertIn("unrated 1", out)

    def test_no_findings_pass(self):
        status, out = run_report(json.dumps({"advisories": {}}), "pnpm")
        self.assertEqual(status, 0)
        self.assertIn("comp: lang 0 vulnerabilities", out)


class AcceptedRiskTest(unittest.TestCase):
    def test_an_accepted_finding_is_listed_but_doesnt_fail(self):
        config = {"audit": {"ignore": [{"id": "GHSA-aaaa-bbbb-0000", "reason": "not called", "until": "2027-01-01"}]}}
        status, out = run_report(finding_report("critical"), "pnpm", config)
        self.assertEqual(status, 0)
        self.assertIn("[accepted until 2027-01-01: not called]", out)
        self.assertIn("0 vulnerabilities", out)
        self.assertIn(", 1 accepted", out)

    def test_an_alias_matches(self):
        config = {"audit": {"ignore": [{"id": "CVE-2020-1000", "reason": "not called", "until": "2027-01-01"}]}}
        self.assertEqual(run_report(finding_report("critical"), "pnpm", config)[0], 0)

    def test_an_expired_entry_stops_applying(self):
        config = {"audit": {"ignore": [{"id": "GHSA-aaaa-bbbb-0000", "reason": "not called", "until": "2026-10-26"}]}}
        status, out = run_report(finding_report("critical"), "pnpm", config)
        self.assertEqual(status, 1)
        self.assertIn("the accepted risk GHSA-aaaa-bbbb-0000 expired on 2026-10-26", out)

    def test_an_entry_lasts_through_its_until_date(self):
        config = {"audit": {"ignore": [{"id": "GHSA-aaaa-bbbb-0000", "reason": "not called", "until": "2026-10-27"}]}}
        self.assertEqual(run_report(finding_report("critical"), "pnpm", config)[0], 0)

    def test_check_wants_a_reason_and_a_date(self):
        problems = audit.config_problems({"audit": {"ignore": [{"id": "GHSA-x"}, {"reason": "r"}],
                                                    "failOn": "severe"}})
        self.assertEqual(problems, ['audit.ignore GHSA-x needs a "reason"',
                                    'audit.ignore GHSA-x needs an "until" date (YYYY-MM-DD)',
                                    'audit.ignore[1] needs an "id"',
                                    "audit.failOn must be one of critical, high, moderate, low"])
        self.assertEqual(audit.config_problems({}), [])


class EndOfLifeTest(unittest.TestCase):
    RELEASES = "3.15.1\t\n3.14.10\t\n3.9.25\teol\n"

    def test_a_line_past_its_end_of_life_warns(self):
        self.assertIn("past its end of life (line 3.9)", audit.eol_warning("python", "3.9.7", self.RELEASES))

    def test_a_supported_line_doesnt(self):
        self.assertIsNone(audit.eol_warning("python", "3.14.2", self.RELEASES))

    def test_lines_match_by_whole_parts(self):
        self.assertIsNone(audit.eol_warning("python", "3.90.1", self.RELEASES))
        self.assertIn("line 16", audit.eol_warning("node", "16.9.1", "24.22.1\tlts\n16.20.2\teol\n", 1))


if __name__ == "__main__":
    unittest.main()


LANGUAGES = os.path.join(HOME, "languages")


def licenses_languages() -> set[str]:
    """The languages hct licenses reads (internal/licenses/licenses.go): those
    with a lock file from a registry it covers."""
    with open(os.path.join(HOME, "internal", "licenses", "licenses.go"), encoding="utf-8") as f:
        source = f.read()
    ecosystems = re.search(r"var Ecosystems = map\[string\]\[\]string\{(.*?)\n\}", source, re.S).group(1)
    covered = re.search(r"var covered = map\[string\]bool\{(.*?)\n\}", source, re.S).group(1)
    registries = set(re.findall(r'"([^"]+)": true', covered))
    return {lang for lang, names in re.findall(r'"(\w+)":\s*\{([^}]*)\}', ecosystems)
            if registries & set(re.findall(r'"([^"]+)"', names))}


def installs_osv_scanner(lang: str) -> bool:
    """Whether the language's installer, or one it always runs (not one it
    runs only when a command is missing), installs OSV-Scanner."""
    with open(os.path.join(LANGUAGES, lang, "install.sh"), encoding="utf-8") as f:
        installer = f.read()
    if re.search(r"^osv_scanner_install$", installer, re.M):
        return True
    delegate = re.search(r'^bash "\$\(dirname "\$0"\)/\.\./(\w+)/install\.sh"', installer, re.M)
    return bool(delegate) and installs_osv_scanner(delegate.group(1))


class OSVScannerInstallTest(unittest.TestCase):
    """OSV-Scanner comes with every language that needs it, so no project
    is left without it."""

    def test_every_language_audited_with_it_installs_it(self):
        for lang in sorted(os.listdir(LANGUAGES)):
            hook = os.path.join(LANGUAGES, lang, "hooks", "audit.sh")
            if not os.path.isfile(hook):
                continue
            with open(hook, encoding="utf-8") as f:
                if "audit_osv" not in f.read():
                    continue
            with self.subTest(lang=lang):
                self.assertTrue(installs_osv_scanner(lang))

    def test_every_language_hct_licenses_reads_installs_it(self):
        langs = licenses_languages()
        self.assertIn("python", langs)
        self.assertNotIn("elixir", langs)
        for lang in sorted(langs):
            with self.subTest(lang=lang):
                self.assertTrue(installs_osv_scanner(lang))
