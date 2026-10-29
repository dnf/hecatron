"""The real-projects run's toolchain caches (tests/real-projects/cache.py)."""

import json
import os
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "real-projects"))
import cache  # noqa: E402
import report  # noqa: E402
from test_real_projects import LOG  # noqa: E402


class KeyTest(unittest.TestCase):
    def test_a_key_names_the_language_and_hecatron_and_follows_the_definition(self):
        with tempfile.TemporaryDirectory() as home:
            os.makedirs(os.path.join(home, "languages", "go"))
            with open(os.path.join(home, "VERSION"), "w") as f:
                f.write("1.2.3\n")
            with open(os.path.join(home, "languages", "go", "config.json"), "w") as f:
                f.write("{}")
            first = cache.key("go", home)
            self.assertTrue(first.startswith("real-projects-go-"), first)
            self.assertIn("-1.2.3-", first)
            self.assertEqual(cache.key("go", home), first)
            with open(os.path.join(home, "languages", "go", "install.json"), "w") as f:
                f.write("{}")
            self.assertNotEqual(cache.key("go", home), first)

    def test_the_cached_directories_are_hecatrons_and_the_definitions_toolchain(self):
        self.assertEqual(cache.toolchain_dirs("go"), cache.HECATRON_DIRS)
        self.assertEqual(cache.toolchain_dirs("haskell"), cache.HECATRON_DIRS + [".ghcup"])


class PlanTest(unittest.TestCase):
    def test_a_plan_restores_each_language_up_to_the_slots(self):
        projects, _ = report.load_all()
        biggest = max(projects["projects"], key=lambda p: len(p["languages"]))["repo"]
        lines = cache.plan(biggest)
        langs = [line.split("=", 1)[1] for line in lines if line.startswith("lang")]
        self.assertLessEqual(len(langs), cache.SLOTS)
        self.assertEqual(len(set(langs)), len(langs))
        self.assertEqual(len(lines), 2 * len(langs))
        with open(os.path.join(os.path.dirname(cache.HERE), "..", ".github", "workflows", "real-projects.yml")) as f:
            workflow = f.read()
        self.assertIn(f"steps.plan.outputs.key{cache.SLOTS} != ''", workflow)
        self.assertNotIn(f"steps.plan.outputs.key{cache.SLOTS + 1} ", workflow)

    def test_single_language_projects_come_first(self):
        projects, results = report.load_all()
        ordered = cache.order()
        self.assertEqual(sorted(ordered), sorted(p["repo"] for p in projects["projects"]))
        count = {repo: len(run["languages"]) for repo, run in results.items()}
        expected = {p["repo"]: count.get(p["repo"].lower()) or len(p["languages"]) for p in projects["projects"]}
        singles = [expected[repo] == 1 for repo in ordered]
        self.assertEqual(singles, sorted(singles, reverse=True))


class SaveRestoreTest(unittest.TestCase):
    def test_a_saved_install_is_restored_whole(self):
        with tempfile.TemporaryDirectory() as tmp:
            store, before, after, archives = (os.path.join(tmp, d) for d in ("cache", "before", "after", "archives"))
            os.makedirs(store)
            os.makedirs(archives)
            toolchain = os.path.join(before, ".local/share/hecatron/toolchains/go/1.25.0/bin")
            os.makedirs(toolchain)
            with open(os.path.join(toolchain, "go"), "w") as f:
                f.write("#!/bin/sh\n")
            os.chmod(os.path.join(toolchain, "go"), 0o755)
            links = os.path.join(before, ".local/share/hecatron/bin")
            os.makedirs(links)
            os.symlink(os.path.join(toolchain, "go"), os.path.join(links, "go"))
            with open(os.path.join(archives, "gcc.deb"), "w") as f:
                f.write("deb")
            cache.save(store, before, "go", archives=archives)

            self.assertEqual(sorted(os.listdir(store)), ["go", "saved"])
            with open(os.path.join(store, "saved")) as f:
                self.assertEqual(f.read(), f"lang=go\nkey={cache.key('go')}\n")
            self.assertEqual(os.listdir(os.path.join(store, "go", "apt")), ["gcc.deb"])

            shutil.rmtree(os.path.join(store, "go", "apt"))  # restore copies debs with sudo
            self.assertEqual(cache.restore(store, after), ["go"])
            restored = os.path.join(after, ".local/share/hecatron/toolchains/go/1.25.0/bin/go")
            self.assertTrue(os.access(restored, os.X_OK))
            self.assertEqual(os.readlink(os.path.join(after, ".local/share/hecatron/bin/go")), os.path.join(toolchain, "go"))

    def test_an_unfinished_cache_isnt_restored(self):
        with tempfile.TemporaryDirectory() as tmp:
            os.makedirs(os.path.join(tmp, "cache", "go", "home"))
            self.assertEqual(cache.restore(os.path.join(tmp, "cache"), os.path.join(tmp, "home")), [])


class RecordTest(unittest.TestCase):
    def test_the_run_records_whether_it_was_cached(self):
        self.assertIsNone(report.parse_log(LOG)["cache"])
        run = report.parse_log('### cache {"enabled": true, "restored": ["go"]}\n' + LOG)
        self.assertEqual(run["cache"], {"enabled": True, "restored": ["go"]})
        with open(report.SCHEMA, encoding="utf-8") as f:
            schema = json.load(f)
        self.assertEqual(report.validate({"schemaVersion": 1, "results": [run]}, schema["$defs"]["resultsFile"], schema), [])


if __name__ == "__main__":
    unittest.main()
