"""The default Node.js test hook (node_hook in languages/_hooks.sh, for
TypeScript and JavaScript): which packages it tests and how, through fake
package managers that print how they were called."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import Fixture, write  # noqa: E402


def package(**scripts: str) -> str:
    return json.dumps({"scripts": scripts})


class NodeTestHookTest(Fixture):
    TOOLS = ("npm", "pnpm", "yarn", "bun", "corepack")

    def setUp(self):
        super().setUp()
        self.bin = self.path(".fakebin")
        for tool in self.TOOLS:
            write(os.path.join(self.bin, tool), f'#!/bin/sh\necho "ran {tool} $* in ${{PWD##*/}}" | tee -a "{self.path(".calls")}"\n', 0o755)

    def files(self, files: dict[str, str]) -> None:
        for name, content in files.items():
            write(self.path(name), content)

    def test(self, lang: str = "typescript", cwd: str = ".", tools=TOOLS, **env) -> subprocess.CompletedProcess:
        fakes = self.path(".fakes")
        os.makedirs(fakes, exist_ok=True)
        for tool in tools:
            target = os.path.join(fakes, tool)
            if not os.path.exists(target):
                os.symlink(os.path.join(self.bin, tool), target)
        path = ":".join([fakes] + [d for d in os.environ["PATH"].split(":") if not self.real_node(d)])
        environ = {**os.environ, "PATH": path, "HECATRON_ROOT": self.root, "HECATRON_LANG": lang,
                   "XDG_CACHE_HOME": self.path(".cache"), **env}
        hook = os.path.join(HOME, "languages", lang, "hooks", "test.sh")
        return subprocess.run(["bash", hook], cwd=self.path(cwd), env=environ, capture_output=True, text=True)

    def real_node(self, directory: str) -> bool:
        """A PATH directory with a real package manager (and not the shell)."""
        return (any(os.path.exists(os.path.join(directory, tool)) for tool in self.TOOLS)
                and not os.path.exists(os.path.join(directory, "bash")))

    def ran(self, result: subprocess.CompletedProcess) -> list[str]:
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return [line[len("ran "):] for line in result.stdout.splitlines() if line.startswith("ran ")]

    def test_a_test_script_runs(self):
        self.files({"package.json": package(test="jest"), "node_modules/.keep": ""})
        self.assertEqual(self.ran(self.test()), ["npm run test in " + os.path.basename(self.root)])

    def test_without_a_package_json_the_languages_packages_below_run(self):
        # TypeScript's packages have a tsconfig.json; a package without a
        # test script, one in tests/ and one inside another are left out.
        self.files({"crates/napi/package.json": package(test="ava"), "crates/napi/tsconfig.json": "{}",
                    "crates/napi/node_modules/.keep": "",
                    "crates/napi/inner/package.json": package(test="x"), "crates/napi/inner/tsconfig.json": "{}",
                    "crates/wasm/package.json": package(test="node --test"),
                    "npm/package.json": package(build="x"), "npm/tsconfig.json": "{}",
                    "tests/fixture/package.json": package(test="x"), "tests/fixture/tsconfig.json": "{}"})
        self.assertEqual(self.ran(self.test("typescript")), ["npm run test in napi"])

    def test_javascripts_packages_below_are_those_without_a_tsconfig(self):
        self.files({"crates/napi/package.json": package(test="ava"), "crates/napi/tsconfig.json": "{}",
                    "crates/wasm/package.json": package(test="node --test"), "crates/wasm/node_modules/.keep": ""})
        self.assertEqual(self.ran(self.test("javascript")), ["npm run test in wasm"])

    def test_a_workspace_root_without_a_test_script_runs_its_packages(self):
        self.files({"package.json": json.dumps({"workspaces": ["packages/*", "!packages/skipped"]}),
                    "node_modules/.keep": "", "yarn.lock": "",
                    "packages/a/package.json": package(test="jest"), "packages/b/package.json": package(build="x"),
                    "packages/skipped/package.json": package(test="x")})
        self.assertEqual(self.ran(self.test()), ["corepack yarn run test in a"])

    def test_a_pnpm_workspace_lists_its_packages_in_its_yaml(self):
        self.files({"package.json": "{}", "node_modules/.keep": "", "pnpm-lock.yaml": "",
                    "pnpm-workspace.yaml": "packages:\n  - 'mantine-ui'\n  - \"module/*\"\nonlyBuiltDependencies: []\n",
                    "mantine-ui/package.json": package(test="vitest"),
                    "module/lezer/package.json": package(test="mocha")})
        self.assertEqual(self.ran(self.test()), ["corepack pnpm run test in mantine-ui",
                                                  "corepack pnpm run test in lezer"])

    def test_a_configured_test_runner_runs_without_a_test_script(self):
        self.files({"package.json": "{}", "jest.config.js": "", "node_modules/.bin/jest": "#!/bin/sh\necho ran jest\n"})
        os.chmod(self.path("node_modules/.bin/jest"), 0o755)
        self.assertEqual(self.ran(self.test()), ["jest"])

    def test_a_root_without_a_test_script_runs_the_packages_below(self):
        self.files({"package.json": package(lint="eslint"), "node/package.json": package(test="jest"),
                    "node/tsconfig.json": "{}", "node/node_modules/.keep": ""})
        self.assertEqual(self.ran(self.test()), ["npm run test in node"])

    def test_no_test_anywhere_fails(self):
        self.files({"package.json": package(build="tsc")})
        result = self.test()
        self.assertEqual(result.returncode, 1)
        self.assertIn("package.json has no test script", result.stderr)

    def test_scripts_find_the_package_manager_through_corepacks_shim(self):
        # Without pnpm on PATH, `pnpm -r test` in a script needs Corepack's shim.
        self.files({"package.json": package(test="pnpm -r test"), "pnpm-lock.yaml": "", "node_modules/.keep": ""})
        self.ran(self.test(tools=("npm", "corepack")))
        with open(self.path(".calls")) as calls:
            first = calls.readline()
        self.assertEqual(first.split(" --install-directory ")[0], "ran corepack enable")

if __name__ == "__main__":
    unittest.main()
