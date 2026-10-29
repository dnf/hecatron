"""The generated CI files the Go core writes parse as YAML, and the GitLab
pipelines match GitLab's own schema for .gitlab-ci.yml (vendored in
tests/schemas/gitlab-ci.json, from gitlab-org/gitlab's
app/assets/javascripts/editor/schema/ci.json, MIT licensed; no network).
The Dependabot configuration matches SchemaStore's schema for it
(tests/schemas/dependabot-2.0.json, from
https://json.schemastore.org/dependabot-2.0.json, Apache-2.0), and every
definition's Dependabot ecosystem is one it lists.
The files checked are internal/generate's golden files, which its Go tests
keep equal to what is generated. Skipped where PyYAML or jsonschema isn't
installed."""

import glob
import json
import os
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GOLDEN = os.path.join(ROOT, "internal", "generate", "testdata", "ci")
SCHEMAS = os.path.join(ROOT, "tests", "schemas")

try:
    import yaml
except ImportError:  # pragma: no cover - depends on the machine
    yaml = None

try:
    import jsonschema
except ImportError:  # pragma: no cover - depends on the machine
    jsonschema = None


def golden(pattern: str) -> list[str]:
    files = sorted(glob.glob(os.path.join(GOLDEN, pattern)))
    if not files:
        raise AssertionError(f"no golden files match {pattern} in {GOLDEN}")
    return files


@unittest.skipIf(yaml is None, "PyYAML is not installed")
class GeneratedCITest(unittest.TestCase):
    def load(self, path: str):
        with open(path, encoding="utf-8") as f:
            return yaml.safe_load(f)

    def test_github_workflows_parse(self):
        for path in golden("github-*.yml"):
            with self.subTest(path=os.path.basename(path)):
                workflow = self.load(path)
                for job in workflow["jobs"].values():
                    steps = job["steps"]
                    checkout = next(i for i, step in enumerate(steps) if step.get("uses", "").startswith("actions/checkout@"))
                    caches = [step for step in steps if step.get("uses", "").startswith("actions/cache@")]
                    self.assertTrue(caches)
                    self.assertTrue(all(steps.index(step) > checkout for step in caches))
                    for step in caches:
                        self.assertTrue(step["with"]["path"].strip())
                        self.assertTrue(step["with"]["key"].startswith(step["with"]["restore-keys"]))

    def test_gitlab_pipelines_parse(self):
        for path in golden("gitlab-*.yml"):
            with self.subTest(path=os.path.basename(path)):
                pipeline = self.load(path)
                jobs = {name: job for name, job in pipeline.items() if name not in ("variables", "default")}
                self.assertTrue(jobs)
                for name, job in jobs.items():
                    if name == "secrets":
                        self.assertEqual(job["script"][-2:], ["hct install secrets", "hct audit secrets"])
                        continue
                    self.assertEqual(job["script"][-3:], [f"hct install {name}", f"hct test {name}", f"hct lint {name}"])
                    for cache in job.get("cache", []):
                        self.assertLessEqual(len(cache["key"].get("files", [])) if isinstance(cache["key"], dict) else 0, 10)
                        for cached in cache["paths"]:
                            self.assertFalse(cached.startswith(("/", "~")), "GitLab caches only paths in the project")

    @unittest.skipIf(jsonschema is None, "jsonschema is not installed")
    def test_gitlab_pipelines_match_gitlabs_schema(self):
        with open(os.path.join(SCHEMAS, "gitlab-ci.json"), encoding="utf-8") as f:
            validator = jsonschema.Draft7Validator(json.load(f))
        for path in golden("gitlab-*.yml"):
            with self.subTest(path=os.path.basename(path)):
                errors = [f"{list(e.path)}: {e.message}" for e in validator.iter_errors(self.load(path))]
                self.assertEqual(errors, [])

    @unittest.skipIf(jsonschema is None, "jsonschema is not installed")
    def test_schema_rejects_a_bad_pipeline(self):
        with open(os.path.join(SCHEMAS, "gitlab-ci.json"), encoding="utf-8") as f:
            validator = jsonschema.Draft7Validator(json.load(f))
        bad = {"go": {"cache": {"key": {"files": [f"f{i}" for i in range(11)]}, "paths": ["x"]}, "script": ["true"]}}
        self.assertTrue(list(validator.iter_errors(bad)))

    def test_dependabot_parses(self):
        config = self.load(os.path.join(GOLDEN, "dependabot.yml"))
        self.assertEqual(config["version"], 2)
        seen = set()
        for update in config["updates"]:
            key = (update["package-ecosystem"], update["directory"])
            self.assertNotIn(key, seen, "one entry per ecosystem and directory")
            seen.add(key)
            self.assertTrue(update["directory"].startswith("/"))
            self.assertIn("interval", update["schedule"])

    @unittest.skipIf(jsonschema is None, "jsonschema is not installed")
    def test_dependabot_matches_its_schema(self):
        with open(os.path.join(SCHEMAS, "dependabot-2.0.json"), encoding="utf-8") as f:
            validator = jsonschema.Draft7Validator(json.load(f))
        errors = [f"{list(e.path)}: {e.message}" for e in validator.iter_errors(self.load(os.path.join(GOLDEN, "dependabot.yml")))]
        self.assertEqual(errors, [])


class DependencyUpdatesTest(unittest.TestCase):
    def definitions(self):
        for path in sorted(glob.glob(os.path.join(ROOT, "languages", "*", "config.json"))):
            with open(path, encoding="utf-8") as f:
                yield os.path.basename(os.path.dirname(path)), json.load(f)

    def test_definitions_name_dependabots_ecosystems(self):
        with open(os.path.join(SCHEMAS, "dependabot-2.0.json"), encoding="utf-8") as f:
            ecosystems = set(json.load(f)["definitions"]["package-ecosystem-values"]["enum"])
        for lang, definition in self.definitions():
            rules = definition.get("dependabot") or []
            for rule in [rules] if isinstance(rules, str) else rules:
                name = rule if isinstance(rule, str) else rule["ecosystem"]
                self.assertIn(name, ecosystems, lang)

    def test_renovate_json(self):
        with open(os.path.join(GOLDEN, "renovate.json"), encoding="utf-8") as f:
            config = json.load(f)
        self.assertEqual(config["$schema"], "https://docs.renovatebot.com/renovate-schema.json")
        self.assertEqual(len(config["enabledManagers"]), len(set(config["enabledManagers"])))
        self.assertTrue(all(path.endswith("/**") for path in config["includePaths"]))
        managers = {m for _, d in self.definitions() for m in d.get("renovate", [])} | {"github-actions"}
        self.assertLessEqual(set(config["enabledManagers"]), managers)


if __name__ == "__main__":
    unittest.main()
