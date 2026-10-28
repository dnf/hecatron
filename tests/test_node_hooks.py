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
import project  # noqa: E402


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
                   "HOME": self.path("home"), **env}
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

    def test_watching_test_runners_run_once(self):
        self.files({"package.json": package(test="react-scripts test"), "node_modules/.keep": ""})
        write(os.path.join(self.bin, "npm"), '#!/bin/sh\necho "ran npm CI=${CI:-}"\n', 0o755)
        self.assertEqual(self.ran(self.test()), ["npm CI=true"])
        self.assertEqual(self.ran(self.test(CI="false")), ["npm CI=false"])

    def test_a_native_addon_is_built_before_its_tests(self):
        self.files({"package.json": json.dumps({"napi": {"binaryName": "x"},
                                                "scripts": {"build": "napi build --platform", "test": "ava"}}),
                    "node_modules/.keep": ""})
        name = os.path.basename(self.root)
        self.assertEqual(self.ran(self.test()), [f"npm run build in {name}", f"npm run test in {name}"])
        write(self.path("x.linux-x64-gnu.node"), "")
        self.assertEqual(self.ran(self.test()), [f"npm run test in {name}"])

    def test_napis_own_build_without_a_build_script(self):
        self.files({"package.json": json.dumps({"napi": {}, "scripts": {"test": "ava"}}),
                    "node_modules/.bin/napi": '#!/bin/sh\necho "ran napi $*"\n'})
        os.chmod(self.path("node_modules/.bin/napi"), 0o755)
        self.assertEqual(self.ran(self.test()),
                         ["napi build --platform", "npm run test in " + os.path.basename(self.root)])

    def test_a_build_script_running_node_gyp_counts_as_native(self):
        self.files({"package.json": package(build="node-gyp rebuild", test="mocha"), "node_modules/.keep": "",
                    "packages/pure/package.json": package(build="tsc", test="x")})
        name = os.path.basename(self.root)
        self.assertEqual(self.ran(self.test()), [f"npm run build in {name}", f"npm run test in {name}"])
        os.makedirs(self.path("build", "Release"))
        write(self.path("build", "Release", "addon.node"), "")
        self.assertEqual(self.ran(self.test()), [f"npm run test in {name}"])

    def test_a_missing_entry_point_is_built_before_the_tests(self):
        self.files({"package.json": json.dumps({"main": "dist/index.cjs", "scripts": {"build": "tsc", "test": "jest"}}),
                    "node_modules/.keep": ""})
        name = os.path.basename(self.root)
        self.assertEqual(self.ran(self.test()), [f"npm run build in {name}", f"npm run test in {name}"])
        write(self.path("dist", "index.cjs"), "")
        self.assertEqual(self.ran(self.test()), [f"npm run test in {name}"])

    def test_an_entry_point_in_exports_counts(self):
        exports = {".": {"types": "./index.d.ts", "import": "./dist/index.mjs"}}
        self.files({"package.json": json.dumps({"exports": exports, "scripts": {"build": "x", "test": "y"}}),
                    "node_modules/.keep": ""})
        self.assertEqual(self.ran(self.test())[0], "npm run build in " + os.path.basename(self.root))

    def test_a_failing_runner_fails_as_itself(self):
        self.files({"package.json": "{}", "jest.config.js": "", "node_modules/.bin/jest": "#!/bin/sh\nexit 3\n"})
        os.chmod(self.path("node_modules/.bin/jest"), 0o755)
        result = self.test()
        self.assertEqual(result.returncode, 3)
        self.assertNotIn("no test command", result.stderr)

    def test_npms_placeholder_test_script_isnt_a_test(self):
        self.files({"package.json": package(test='echo "Error: no test specified" && exit 1')})
        result = self.test()
        self.assertEqual(result.returncode, 1)
        self.assertIn("package.json has no test script", result.stderr)

    def test_a_workspace_package_its_tests_use_is_built_first(self):
        # Prometheus: mantine-ui's tests import codemirror-promql, whose dist/
        # the workspace's build writes.
        self.files({"pnpm-workspace.yaml": "packages:\n  - mantine-ui\n  - module/*\n", "pnpm-lock.yaml": "",
                    "package.json": "{}", "mantine-ui/package.json": package(test="vitest"),
                    "mantine-ui/node_modules/.keep": "",
                    "module/promql/package.json": json.dumps({"main": "dist/index.cjs", "scripts": {"build": "x"}})})
        self.assertEqual(self.ran(self.test(cwd="mantine-ui")),
                         ["corepack pnpm -r --if-present run build in " + os.path.basename(self.root),
                          "corepack pnpm run test in mantine-ui"])

    def test_a_workspace_root_with_a_test_script_builds_its_native_packages(self):
        self.files({"package.json": json.dumps({"workspaces": ["packages/*"], "scripts": {"test": "x"}}),
                    "package-lock.json": "{}", "node_modules/.keep": "",
                    "packages/core/package.json": json.dumps({"napi": {}, "scripts": {"build": "napi build"}})})
        name = os.path.basename(self.root)
        self.assertEqual(self.ran(self.test()), [f"npm run build --workspaces --if-present in {name}",
                                                  f"npm run test in {name}"])
        write(self.path("packages", "core", "core.linux-x64-gnu.node"), "")
        self.assertEqual(self.ran(self.test()), [f"npm run test in {name}"])

    def test_a_package_without_a_native_addon_isnt_built(self):
        self.files({"package.json": package(build="tsc", test="jest"), "node_modules/.keep": ""})
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
        self.assertEqual(first.split(" --install-directory ")[1].split()[0],
                         self.path("home", ".cache", "hecatron", "corepack-shims"))

    def test_other_languages_hooks_have_the_shims_on_path(self):
        # swc's Rust tests run pnpm: the shim directory is a static "path"
        # entry of the Node.js languages, which other languages' hooks get.
        os.makedirs(self.path("home", ".cache", "hecatron", "corepack-shims"))
        self.configure(languages=["rust", "typescript"])
        saved = os.environ.get("HOME")
        os.environ["HOME"] = self.path("home")
        try:
            path = project.load(self.root).language_env("rust")["path"]
        finally:
            os.environ["HOME"] = saved
        self.assertIn(self.path("home", ".cache", "hecatron", "corepack-shims"), path)

if __name__ == "__main__":
    unittest.main()
