"""hecatron deps: the deps action, the fetch functions it shares with the
hooks that install dependencies when they're missing (languages/_hooks.sh,
languages/_deps.sh), and the installers' route through it (impl_deps).
Each ecosystem's tool is a fake on PATH that prints how it was called; the
real tools run in containers (tests/deps/check.sh)."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import unittest

HOME = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(HOME, "tests"))

from test_hecatron import BIN, Fixture, write  # noqa: E402

HOOKS = os.path.join(HOME, "languages", "_hooks.sh")
DEPS = os.path.join(HOME, "languages", "_deps.sh")
PACKAGE_MANAGER = os.path.join(HOME, "languages", "typescript", "package-manager.sh")
COMMON = os.path.join(HOME, "languages", "_common.sh")


class DepsCommandTest(Fixture):
    def setUp(self):
        super().setUp()
        self.configure(languages=["fixture"], components=["a", "b"], implPath="{component}/{lang}")
        for component in ("a", "b"):
            os.makedirs(self.path(component, "fixture"))
        write(self.path(".hecatron", "languages", "fixture", "config.json"), json.dumps({"dependency": "bash"}))
        write(self.path(".hecatron", "languages", "fixture", "deps.sh"),
              '#!/usr/bin/env bash\necho "fetched $1"\n', 0o755)

    def test_deps_runs_for_every_component(self):
        result = self.hecatron("deps", "fixture")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("fetched a", result.stdout)
        self.assertIn("fetched b", result.stdout)

    def test_a_language_without_a_dependency_manager_is_skipped(self):
        os.remove(self.path(".hecatron", "languages", "fixture", "deps.sh"))
        result = self.hecatron("deps", "fixture")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("No dependency manager for fixture", result.stdout)

    def test_installers_fetch_through_the_deps_hook(self):
        script = f"source {COMMON}\nimpl_deps fixture"
        env = {**os.environ, "HECATRON_HOME": HOME, "HECATRON_ROOT": self.root}
        result = subprocess.run(["bash", "-c", script], cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("fetched a", result.stdout)
        self.assertIn("fetched b", result.stdout)

    def test_installers_skip_a_language_the_project_doesnt_have(self):
        # TypeScript's installer also runs for JavaScript, which shares Node.js.
        script = f"source {COMMON}\nimpl_deps typescript && echo skipped"
        env = {**os.environ, "HECATRON_HOME": HOME, "HECATRON_ROOT": self.root}
        result = subprocess.run(["bash", "-c", script], cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "skipped")


class FetchTest(Fixture):
    """What each fetch runs, through fake tools that print their arguments
    (and BUNDLE_FROZEN)."""

    TOOLS = ("npm", "pnpm", "yarn", "bun", "corepack", "uv", "cargo", "go", "bundle", "dart", "flutter",
             "composer", "mix", "dotnet", "shards")

    def setUp(self):
        super().setUp()
        self.bin = self.path(".fakebin")
        for tool in self.TOOLS:
            write(os.path.join(self.bin, tool),
                  f'#!/bin/sh\ncase " $* " in *" --version "*) echo "${{FAKE_VERSION:-1.0.0}}"; exit 0 ;; esac\n'
                  f'echo "{tool} $* frozen=${{BUNDLE_FROZEN:-}}"\n', 0o755)

    def shell(self, script: str, cwd: str = ".", **env) -> subprocess.CompletedProcess:
        environ = {**os.environ, "PATH": f"{self.bin}:{os.environ['PATH']}", "HECATRON_ROOT": self.root,
                   "HECATRON_LANG": "fixture", **env}
        return subprocess.run(["bash", "-c", f"source {HOOKS}\nsource {DEPS}\n{script}"],
                              cwd=self.path(cwd), env=environ, capture_output=True, text=True)

    def ran(self, result: subprocess.CompletedProcess) -> list[str]:
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return [line.split(" frozen=")[0] for line in result.stdout.splitlines() if " frozen=" in line]

    def node(self, files: dict, cwd: str = ".", **env) -> list[str]:
        for name, content in files.items():
            write(self.path(name), content)
        return self.ran(self.shell("deps_node", cwd=cwd, **env))

    def test_npm_installs_from_its_lock_with_ci(self):
        self.assertEqual(self.node({"package.json": "{}", "package-lock.json": "{}"}), ["npm ci"])

    def test_npm_without_a_lock_installs(self):
        self.assertEqual(self.node({"package.json": "{}"}), ["npm install"])

    def test_pnpm_holds_to_the_workspace_lock(self):
        ran = self.node({"pnpm-lock.yaml": "", "package.json": "{}", "packages/a/package.json": "{}"}, cwd="packages/a")
        self.assertEqual(ran, ["corepack pnpm install --frozen-lockfile"])

    def test_yarn_classic_and_berry_have_their_own_flags(self):
        files = {"package.json": json.dumps({"packageManager": "yarn@1.22.22"}), "yarn.lock": ""}
        self.assertEqual(self.node(files, FAKE_VERSION="1.22.22"), ["corepack yarn install --frozen-lockfile"])
        self.assertEqual(self.node(files, FAKE_VERSION="4.5.0"), ["corepack yarn install --immutable"])

    def test_bun_holds_to_its_lock(self):
        self.assertEqual(self.node({"package.json": "{}", "bun.lock": ""}), ["bun install --frozen-lockfile"])

    def test_the_test_hooks_fetch_only_what_is_missing(self):
        write(self.path("package.json"), "{}")
        script = f"source {PACKAGE_MANAGER}\nnode_pm_command\nnpm_deps\nmkdir node_modules\nnpm_deps"
        self.assertEqual(self.ran(self.shell(script)), ["npm install"])

    def test_python_syncs_locked_with_a_uv_lock(self):
        write(self.path("pyproject.toml"), "")
        self.assertEqual(self.ran(self.shell("deps_python")), ["uv sync --all-extras"])
        write(self.path("uv.lock"), "")
        self.assertEqual(self.ran(self.shell("deps_python")), ["uv sync --locked --all-extras"])

    def test_python_requirements_go_into_a_venv(self):
        write(self.path("requirements.txt"), "requests==2.0\n")
        self.assertEqual(self.ran(self.shell("deps_python")),
                         ["uv venv --allow-existing", "uv pip install -r requirements.txt"])

    def test_rust_fetches_locked_with_the_workspace_lock(self):
        write(self.path("Cargo.toml"), "[workspace]\n")
        write(self.path("crates", "a", "Cargo.toml"), "[package]\n")
        self.assertEqual(self.ran(self.shell("deps_rust", cwd="crates/a")), ["cargo fetch"])
        write(self.path("Cargo.lock"), "")
        self.assertEqual(self.ran(self.shell("deps_rust", cwd="crates/a")), ["cargo fetch --locked"])

    def test_ruby_installs_frozen_to_its_lock(self):
        write(self.path("Gemfile"), "")
        result = self.shell("deps_ruby")
        self.assertIn("frozen=\n", result.stdout + "\n")
        write(self.path("Gemfile.lock"), "")
        self.assertIn("bundle install frozen=true", self.shell("deps_ruby").stdout)

    def test_dart_enforces_its_lock(self):
        write(self.path("pubspec.yaml"), "name: x\n")
        self.assertEqual(self.ran(self.shell("deps_dart")), ["dart pub get"])
        write(self.path("pubspec.lock"), "")
        self.assertEqual(self.ran(self.shell("deps_dart")), ["dart pub get --enforce-lockfile"])

    def test_dotnet_restores_in_locked_mode_with_lock_files(self):
        write(self.path("App.csproj"), "<Project/>")
        self.assertEqual(self.ran(self.shell("deps_dotnet")), ["dotnet restore"])
        write(self.path("packages.lock.json"), "{}")
        self.assertEqual(self.ran(self.shell("deps_dotnet")), ["dotnet restore --locked-mode"])

    def test_go_and_the_lock_owning_tools(self):
        write(self.path("go.mod"), "module x\n")
        write(self.path("composer.json"), "{}")
        write(self.path("mix.exs"), "")
        self.assertEqual(self.ran(self.shell("deps_go")), ["go mod download"])
        self.assertEqual(self.ran(self.shell("deps_php")), ["composer install --no-interaction"])
        self.assertEqual(self.ran(self.shell("deps_elixir")), ["mix deps.get"])

    def test_nothing_to_fetch_skips(self):
        result = self.shell("deps_go")
        self.assertEqual(result.returncode, 0)
        self.assertIn("no go.mod (skipping)", result.stdout)


class HooksTest(unittest.TestCase):
    def test_every_deps_hook_is_executable(self):
        languages = os.path.join(HOME, "languages")
        hooks = [os.path.join(languages, lang, "hooks", "deps.sh") for lang in os.listdir(languages)]
        hooks = [hook for hook in hooks if os.path.isfile(hook)]
        self.assertGreater(len(hooks), 20)
        for hook in hooks:
            self.assertTrue(os.access(hook, os.X_OK), hook)

    def test_help_lists_deps(self):
        result = subprocess.run(["bash", BIN, "help"], capture_output=True, text=True)
        self.assertIn("deps [lang...]", result.stdout)


if __name__ == "__main__":
    unittest.main()
