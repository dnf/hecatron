"""The bills of materials hct sbom writes match the official JSON schemas:
CycloneDX 1.6 (tests/schemas/cyclonedx-bom-1.6.json, with the
cyclonedx-spdx.json and cyclonedx-jsf-0.82.json it refers to, from
https://github.com/CycloneDX/specification/tree/1.6/schema, Apache-2.0) and
SPDX 2.3 (tests/schemas/spdx-2.3.json, from
https://github.com/spdx/spdx-spec/blob/support/2.3/schemas/spdx-schema.json);
no network. The files checked are internal/sbom's golden files, the license
fixtures' SBOMs (tests/licenses/fixtures, with OSV-Scanner's captured
reports), which its Go tests keep equal to what is written. Skipped where
jsonschema isn't installed."""

import glob
import json
import os
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GOLDEN = os.path.join(ROOT, "internal", "sbom", "testdata")
SCHEMAS = os.path.join(ROOT, "tests", "schemas")

try:
    import jsonschema
    from referencing import Registry, Resource
except ImportError:  # pragma: no cover - depends on the machine
    jsonschema = None


def load(path: str):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def golden(suffix: str) -> list[str]:
    return sorted(glob.glob(os.path.join(GOLDEN, "*" + suffix)))


@unittest.skipIf(jsonschema is None, "jsonschema is not installed")
class CycloneDXTest(unittest.TestCase):
    def validator(self):
        schemas = [load(os.path.join(SCHEMAS, name))
                   for name in ("cyclonedx-bom-1.6.json", "cyclonedx-spdx.json", "cyclonedx-jsf-0.82.json")]
        registry = Registry().with_resources((s["$id"], Resource.from_contents(s)) for s in schemas)
        return jsonschema.Draft7Validator(schemas[0], registry=registry,
                                          format_checker=jsonschema.Draft7Validator.FORMAT_CHECKER)

    def test_golden_boms_match_the_schema(self):
        files = golden(".cdx.json")
        self.assertGreaterEqual(len(files), 2)
        validator = self.validator()
        for path in files:
            with self.subTest(os.path.basename(path)):
                errors = [f"{list(e.absolute_path)}: {e.message}" for e in validator.iter_errors(load(path))]
                self.assertEqual(errors, [])

    def test_schema_rejects_a_bad_bom(self):
        bom = load(golden(".cdx.json")[0])
        bom["components"][0]["scope"] = "sometimes"
        self.assertTrue(list(self.validator().iter_errors(bom)))


@unittest.skipIf(jsonschema is None, "jsonschema is not installed")
class SPDXTest(unittest.TestCase):
    def validator(self):
        return jsonschema.Draft201909Validator(load(os.path.join(SCHEMAS, "spdx-2.3.json")))

    def test_golden_documents_match_the_schema(self):
        files = golden(".spdx.json")
        self.assertGreaterEqual(len(files), 2)
        validator = self.validator()
        for path in files:
            with self.subTest(os.path.basename(path)):
                errors = [f"{list(e.absolute_path)}: {e.message}" for e in validator.iter_errors(load(path))]
                self.assertEqual(errors, [])

    def test_schema_rejects_a_bad_document(self):
        document = load(golden(".spdx.json")[0])
        document["relationships"][0]["relationshipType"] = "LIKES"
        self.assertTrue(list(self.validator().iter_errors(document)))


if __name__ == "__main__":
    unittest.main()
