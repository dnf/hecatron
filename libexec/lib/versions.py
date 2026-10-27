"""Toolchain versions (docs/design.md §18): hecatron's version syntax, how
versions compare, and the ecosystem pin files a project can choose one with.

A spec is either a prefix, "3.12", meaning the newest release whose version
begins with those whole components (3.12, 3.12.7, but not 3.120), or exact,
"=4.5", meaning the release named 4.5 and nothing else. A leading "v" and
build metadata ("+11") are ignored; a pre-release ("-rc1") is its own version
and sorts before its release.
"""

from __future__ import annotations

import json
import re

_VERSION = re.compile(r"^v?(\d+(?:\.\d+)*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$")


def _parse(text: str) -> tuple[tuple[int, ...], str] | None:
    """(numeric components, pre-release suffix) for a version, or None."""
    match = _VERSION.match(text.strip())
    if not match:
        return None
    return tuple(int(part) for part in match.group(1).split(".")), match.group(2) or ""


def is_version(text: str) -> bool:
    return _parse(text) is not None


def sort_key(text: str) -> tuple:
    """Orders versions numerically; a pre-release sorts just before its release."""
    parsed = _parse(text)
    if parsed is None:
        return ((), 0, text)
    numbers, pre = parsed
    return (numbers, 0 if pre else 1, pre)


def newest(candidates: list[str]) -> str | None:
    return max(candidates, key=sort_key) if candidates else None


class Spec:
    """A version as a project or definition writes it: "3.12" or "=4.5"."""

    def __init__(self, text: str):
        self.text = text
        self.exact = text.startswith("=")
        body = text[1:] if self.exact else text
        parsed = _parse(body)
        if parsed is None:
            raise ValueError(f"not a version: {text!r}")
        self.numbers, self.pre = parsed

    def matches(self, version: str) -> bool:
        parsed = _parse(version)
        if parsed is None:
            return False
        numbers, pre = parsed
        if self.exact:
            return numbers == self.numbers and pre == self.pre
        return numbers[:len(self.numbers)] == self.numbers and (not self.pre or pre == self.pre)

    def __str__(self) -> str:
        return self.text


# -- pin files ----------------------------------------------------------------

# A pin file that doesn't concern this language (a go.mod without a toolchain
# line, a .tool-versions without the language's key): resolution keeps looking.
NOT_PINNED = object()


def _plain(value: str | None, exact: bool = False):
    """A version from an ecosystem file as a spec; None for a channel name
    like "stable" or "lts/*", which only the ecosystem's own tool resolves."""
    if value is None:
        return None
    value = value.strip().strip("\"'")
    if not is_version(value):
        return None
    value = value.lstrip("v")
    return f"={value}" if exact else value


def _first_line(text: str) -> str | None:
    for line in text.splitlines():
        line = line.split("#", 1)[0].strip()
        if line:
            return line
    return None


def _toml_value(text: str, section: str, key: str):
    """A string (or the first of a list, or a table's "version") under
    [section] in a small TOML file. Hecatron reads only these simple forms."""
    current = None
    for raw in text.splitlines():
        line = raw.split("#", 1)[0].strip()
        if line.startswith("[") and line.endswith("]"):
            current = line.strip("[]").strip()
            continue
        if current != section or "=" not in line:
            continue
        name, value = (part.strip() for part in line.split("=", 1))
        if name.strip("\"'") != key:
            continue
        if value.startswith("["):
            items = re.findall(r"[\"']([^\"']*)[\"']", value)
            return items[0] if items else None
        if value.startswith("{"):
            match = re.search(r"version\s*=\s*[\"']([^\"']*)[\"']", value)
            return match.group(1) if match else None
        return value.strip("\"'")
    return NOT_PINNED


def _python_version(text, key):
    return _plain(_first_line(text))


def _node_version(text, key):
    return _plain(_first_line(text))


def _ruby_version(text, key):
    line = _first_line(text)
    return _plain(line[len("ruby-"):] if line and line.startswith("ruby-") else line)


def _java_version(text, key):
    # jenv names: "17", "17.0.2", "openjdk64-17.0.2", "temurin64-21.0.2"
    line = _first_line(text) or ""
    match = re.search(r"(\d+(?:\.\d+)*)$", line)
    return _plain(match.group(1)) if match else None


def _rust_toolchain_toml(text, key):
    channel = _toml_value(text, "toolchain", "channel")
    return None if channel is NOT_PINNED else _plain(channel, exact=True)


def _rust_toolchain(text, key):
    # The legacy file holds a bare channel, or the same TOML as the new one.
    if "[toolchain]" in text:
        return _rust_toolchain_toml(text, key)
    return _plain(_first_line(text), exact=True)


# How many leading parts of global.json's sdk.version each rollForward policy
# keeps. The SDK accepts any version at or above the one named within that
# scope, and the newest release there always qualifies. "patch", the default,
# keeps the feature band (10.0.1xx), which a prefix can't name, so the
# version stays as written. Under "major" and "latestMajor" the newest release
# of the named major qualifies too, so that's what a component needs, though
# the SDK may run a newer major's.
_ROLL_FORWARD_PARTS = {
    "disable": None,
    "patch": 3,
    "latestPatch": 3,
    "feature": 2,
    "latestFeature": 2,
    "minor": 1,
    "latestMinor": 1,
    "major": 1,
    "latestMajor": 1,
}


def _global_json(text, key):
    try:
        sdk = json.loads(text).get("sdk") or {}
    except (json.JSONDecodeError, AttributeError):
        return None
    if "version" not in sdk:
        return NOT_PINNED
    policy = sdk.get("rollForward", "patch")
    if policy == "disable" or policy not in _ROLL_FORWARD_PARTS:
        return _plain(sdk["version"], exact=policy == "disable")
    spec = _plain(sdk["version"])
    # A pre-release (11.0.100-rc.1...) is named in full: its line has no
    # release yet.
    if spec is None or "-" in spec:
        return spec
    return ".".join(spec.split(".")[:_ROLL_FORWARD_PARTS[policy]])


def _composer_json(text, key):
    # config.platform.php is the PHP version Composer resolves dependencies
    # for, so the project's tools expect it: the newest release on its line.
    # The require constraint on php ("^8.1") isn't a pin: it's the range a
    # library supports, which the default normally falls in, so a component
    # keeps the default (the PHP doctor checks the constraint holds).
    try:
        config = json.loads(text).get("config") or {}
        value = (config.get("platform") or {}).get("php")
    except (json.JSONDecodeError, AttributeError):
        return NOT_PINNED
    if value is None:
        return NOT_PINNED
    spec = _plain(str(value))
    return ".".join(spec.split(".")[:2]) if spec else None


def _go_mod_toolchain(text, key):
    match = re.search(r"^\s*toolchain\s+go(\S+)\s*$", text, re.MULTILINE)
    return _plain(match.group(1), exact=True) if match else NOT_PINNED


def _gradle_daemon_jvm(text, key):
    match = re.search(r"^\s*toolchainVersion\s*=\s*(\S+)\s*$", text, re.MULTILINE)
    return _plain(match.group(1)) if match else NOT_PINNED


def _tool_versions(text, key):
    for line in text.splitlines():
        fields = line.split("#", 1)[0].split()
        if len(fields) >= 2 and fields[0] == key:
            return _plain(fields[1])
    return NOT_PINNED


def _mise(text, key):
    value = _toml_value(text, "tools", key)
    return value if value is NOT_PINNED else _plain(value)


def _sdkmanrc(text, key):
    for line in text.splitlines():
        line = line.split("#", 1)[0].strip()
        if "=" in line:
            name, value = (part.strip() for part in line.split("=", 1))
            if name == (key or "java"):
                # SDKMAN! identifiers add a vendor: 21.0.2-tem
                return _plain(re.sub(r"-[A-Za-z][\w.]*$", "", value))
    return NOT_PINNED


FORMATS = {
    "python-version": _python_version,
    "nvmrc": _node_version,
    "node-version": _node_version,
    "ruby-version": _ruby_version,
    "php-version": _python_version,
    "composer-json": _composer_json,
    "swift-version": _python_version,
    "java-version": _java_version,
    "rust-toolchain": _rust_toolchain,
    "rust-toolchain-toml": _rust_toolchain_toml,
    "global-json": _global_json,
    "go-mod-toolchain": _go_mod_toolchain,
    "gradle-daemon-jvm": _gradle_daemon_jvm,
    "tool-versions": _tool_versions,
    "mise": _mise,
    "sdkmanrc": _sdkmanrc,
}


def read_pin(fmt: str, text: str, key: str | None = None):
    """A pin file's version as a spec, None when it names no version (a
    channel like "stable"), or NOT_PINNED when it doesn't pin this language."""
    if fmt not in FORMATS:
        raise ValueError(f"unknown pin file format {fmt!r} (known: {', '.join(sorted(FORMATS))})")
    return FORMATS[fmt](text, key)
