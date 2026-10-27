"""The Agent Skill (plugins/hecatron/skills/hecatron/): generated parts
current, frontmatter valid by the Agent Skills specification."""

import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "libexec", "codegen"))
import skill  # noqa: E402

VALID = "---\nname: hecatron\ndescription: Runs tests.\nlicense: MIT\n---\n\nBody\n"


class SkillTest(unittest.TestCase):
    def test_generated_files_are_current(self):
        self.assertEqual(skill.main(["--check"]), 0, "run python3 libexec/codegen/skill.py")

    def test_frontmatter_problems(self):
        self.assertEqual(skill.problems(VALID), [])
        self.assertTrue(skill.problems(VALID.replace("license: MIT", "when_to_use: tests")))
        self.assertTrue(skill.problems(VALID.replace("name: hecatron", "name: Hecatron")))
        self.assertTrue(skill.problems(VALID.replace("Runs tests.", "x" * 1025)))
        self.assertTrue(skill.problems("no frontmatter"))

    def test_only_commands_that_run_nothing_of_the_project_are_preapproved(self):
        with open(os.path.join(skill.SKILL, "SKILL.md"), encoding="utf-8") as f:
            allowed = next(line for line in f if line.startswith("allowed-tools:"))
        for command in ("install", "--fix", "hct test", "hct build", "hct lint", "hct deps", "hct format"):
            self.assertNotIn(command, allowed)


if __name__ == "__main__":
    unittest.main()
