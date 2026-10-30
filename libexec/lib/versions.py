"""Toolchain versions (docs/guide/versions.md): hecatron's version syntax, how
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

# A spec with this prefix is a floor, not a pin: any version at least this
# one will do (global.json's latestMajor, a project's target framework).
FLOOR = ">="


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
    if policy == "latestMajor":
        return FLOOR + spec
    return ".".join(spec.split(".")[:_ROLL_FORWARD_PARTS[policy]])


def _target_framework(text, key):
    # An MSBuild project's TargetFramework(s) (net8.0;net10.0): the newest
    # .NET it builds for needs an SDK of that major or later. Older names
    # (netstandard2.0, netcoreapp3.1, net472) don't raise one.
    majors = [int(major) for value in re.findall(r"<TargetFrameworks?>([^<]*)<", text)
              for major in re.findall(r"(?:^|[^A-Za-z0-9_.])net(\d+)\.\d+", value) if int(major) >= 5]
    return FLOOR + str(max(majors)) if majors else NOT_PINNED


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


def _npm_engines(text, key):
    # package.json's engines.node (">=22"): not a pin but the range of
    # versions the package runs on, for rangeFiles (the range, normalized,
    # or NOT_PINNED for none or one this can't read).
    try:
        engines = json.loads(text).get("engines") or {}
        value = engines.get(key or "node")
    except (json.JSONDecodeError, AttributeError):
        return NOT_PINNED
    if not isinstance(value, str) or parse_range(value) is None:
        return NOT_PINNED
    return " ".join(value.split())


def _go_mod_toolchain(text, key):
    match = re.search(r"^\s*toolchain\s+go(\S+)\s*$", text, re.MULTILINE)
    return _plain(match.group(1), exact=True) if match else NOT_PINNED


def _build_zig_zon(text, key):
    # Zig breaks compatibility between minor releases (0.14 code doesn't
    # build with 0.15), so minimum_zig_version names its release line, the
    # newest of which the build works with, not a floor the default meets.
    match = re.search(r'^[^/\n]*?\.minimum_zig_version\s*=\s*"([^"]*)"', text, re.MULTILINE)
    if not match:
        return NOT_PINNED
    spec = _plain(match.group(1))
    return ".".join(re.split(r"[-+]", spec, maxsplit=1)[0].split(".")[:2]) if spec else None


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


def _python_classifiers(text, key):
    # The trove classifiers a pyproject.toml, setup.cfg or setup.py lists
    # ("Programming Language :: Python :: 3.14"): the newest Python the
    # project says it supports, a line (3.14), not a pin (docs/guide/versions.md).
    found = re.findall(r"Programming Language :: Python :: (\d+\.\d+)\b", text)
    return _plain(newest(found)) if found else NOT_PINNED


# A GitHub Actions setup step's version input ("node-version: 16.x",
# "java-version: ['11', '17']", "python-version: ${{ matrix.python }}"): the
# newest version the project's CI runs the language on, a line hecatron holds
# the default back to. Read without a YAML parser: the input's values, inline
# or as a list, and a matrix variable's values wherever the file sets it.
# Inputs whose bare major says nothing ("python-version: 3.x") need two parts.
_SETUP_PARTS = {"python-version": 2, "go-version": 2, "ruby-version": 2, "php-version": 2}


def _yaml_values(text: str, key: str) -> list[str]:
    """Every value a key is given in a workflow: inline, a [list], or the
    "- item" lines under it."""
    values = []
    lines = text.splitlines()
    pattern = re.compile(rf"^(\s*)(?:-\s+)?{re.escape(key)}\s*:\s*(.*?)\s*$")
    for i, line in enumerate(lines):
        match = pattern.match(line)
        if not match:
            continue
        value = re.sub(r"\s+#.*$", "", match.group(2))
        if value.startswith("["):
            values += [item.strip() for item in value.strip("[]").split(",")]
        elif value:
            values.append(value)
        else:
            for item in lines[i + 1:]:
                listed = re.match(r"^\s*-\s+(\S+)\s*$", item)
                if not listed:
                    break
                values.append(listed.group(1))
    return values


def _setup_version(value: str, key: str) -> str | None:
    value = value.strip().strip("\"'")
    while value.endswith(".x"):
        value = value[:-2]
    if key == "java-version" and value.startswith("1."):
        value = value[2:]  # 1.8 is Java 8
    if not re.fullmatch(r"\d+(\.\d+)*", value) or len(value.split(".")) < _SETUP_PARTS.get(key, 1):
        return None
    return value


def _github_setup(text, key):
    found = []
    for value in _yaml_values(text, key):
        matrix = re.fullmatch(r"\$\{\{\s*matrix\.([\w-]+)\s*\}\}", value.strip("\"'"))
        candidates = [v for v in _yaml_values(text, matrix.group(1)) if "${{" not in v] if matrix else [value]
        found += [v for v in (_setup_version(c, key) for c in candidates) if v]
    return _plain(newest(found)) if found else NOT_PINNED


# javac's oldest --source/--target levels: JDK 12 dropped 6, JDK 20 dropped
# 7, so a Maven build at level 6 or below needs JDK 11 at the newest, and one
# at 7 needs 19. Level 8 still compiles on today's JDKs.
_NEWEST_JDK_FOR_LEVEL = ((6, "11"), (7, "19"))


def _maven_source_level(text, key):
    levels = []
    for value in re.findall(r"<(?:maven\.compiler\.)?(?:source|target|release)>\s*([\d.]+)\s*</", text):
        level = value[2:] if value.startswith("1.") else value
        if level.isdigit():
            levels.append(int(level))
    for oldest, jdk in _NEWEST_JDK_FOR_LEVEL:
        if levels and min(levels) <= oldest:
            return jdk
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
    "build-zig-zon": _build_zig_zon,
    "gradle-daemon-jvm": _gradle_daemon_jvm,
    "tool-versions": _tool_versions,
    "mise": _mise,
    "sdkmanrc": _sdkmanrc,
    "python-classifiers": _python_classifiers,
    "github-setup": _github_setup,
    "maven-source-level": _maven_source_level,
    "target-framework": _target_framework,
    "npm-engines": _npm_engines,
}


# -- ranges -------------------------------------------------------------------

# A range is npm's (node-semver's) syntax, which package.json's engines use:
# comparators (>=22, <23.1, =22.4.0), partial versions (22, 22.x, *), ~ and ^,
# hyphens (20 - 22), all of a space-separated set, any set of ||. Parsed into
# sets of (operator, (major, minor, patch)) comparators.
_PARTIAL = re.compile(r"^v?(\d+|[xX*])?(?:\.(\d+|[xX*]))?(?:\.(\d+|[xX*]))?(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$")
_OPERATOR = re.compile(r"^(<=|>=|<|>|=|~>|~|\^)?(.*)$")


def _partial(text: str) -> list[int] | None:
    """The numbers a partial version gives, up to the first wildcard."""
    match = _PARTIAL.match(text)
    if not match:
        return None
    numbers = []
    for part in match.groups():
        if part is None or part in "xX*":
            break
        numbers.append(int(part))
    return numbers


def _padded(numbers: list[int]) -> tuple[int, int, int]:
    return tuple((numbers + [0, 0, 0])[:3])


def _next(numbers: list[int]) -> tuple[int, int, int]:
    """The first version past a partial one: 22 -> 23.0.0, 22.4 -> 22.5.0."""
    bumped = numbers[:-1] + [numbers[-1] + 1]
    return _padded(bumped)


def _comparators(token: str) -> list | None:
    operator, rest = _OPERATOR.match(token).groups()
    numbers = _partial(rest)
    if numbers is None:
        return None
    n = len(numbers)
    if n == 0:
        return [("<", (0, 0, 0))] if operator in ("<", ">") else []
    low = _padded(numbers)
    if operator in (None, "="):
        return [(">=", low), ("<=", low)] if n == 3 else [(">=", low), ("<", _next(numbers))]
    if operator == ">=":
        return [(">=", low)]
    if operator == ">":
        return [(">", low)] if n == 3 else [(">=", _next(numbers))]
    if operator == "<":
        return [("<", low)]
    if operator == "<=":
        return [("<=", low)] if n == 3 else [("<", _next(numbers))]
    if operator in ("~", "~>"):
        return [(">=", low), ("<", _next(numbers[:2]))]
    # ^: the leftmost non-zero part stays fixed (^0.2.3 is below 0.3.0).
    fixed = next((i for i, part in enumerate(numbers) if part != 0), n - 1)
    return [(">=", low), ("<", _next(numbers[:fixed + 1]))]


def _hyphen(low: str, high: str) -> list | None:
    first, last = _partial(low), _partial(high)
    if first is None or last is None:
        return None
    bounds = [(">=", _padded(first))] if first else []
    if len(last) == 3:
        bounds.append(("<=", _padded(last)))
    elif last:
        bounds.append(("<", _next(last)))
    return bounds


def parse_range(text: str) -> list | None:
    """A range's sets of comparators, or None when it isn't one."""
    sets = []
    for alternative in text.split("||"):
        alternative = re.sub(r"(<=|>=|<|>|=|~>|~|\^)\s+", r"\1", alternative.strip())
        hyphen = re.fullmatch(r"(\S+)\s+-\s+(\S+)", alternative)
        if hyphen:
            bounds = _hyphen(hyphen.group(1), hyphen.group(2))
        else:
            bounds = []
            for token in alternative.split():
                found = _comparators(token)
                if found is None:
                    return None
                bounds += found
        if bounds is None:
            return None
        sets.append(bounds)
    return sets


def satisfies(text: str, version: str) -> bool:
    """Whether a release is in a range; a pre-release never is."""
    sets, parsed = parse_range(text), _parse(version)
    if sets is None or parsed is None or parsed[1]:
        return False
    numbers = _padded(list(parsed[0]))
    tests = {"<": numbers.__lt__, "<=": numbers.__le__, ">": numbers.__gt__, ">=": numbers.__ge__}
    return any(all(tests[operator](bound) for operator, bound in bounds) for bounds in sets)


def strip_pin(spec, pattern: str | None):
    """A pin's spec with an entry's "strip" (a regular expression) removed:
    asdf's and mise's Elixir "1.17.3-otp-27" is Elixir 1.17.3, built for
    OTP 27. None when what's left names no version."""
    if pattern is None or spec is None or spec is NOT_PINNED:
        return spec
    exact = spec.startswith("=")
    value = re.sub(pattern, "", spec[1:] if exact else spec)
    if not is_version(value):
        return None
    return ("=" if exact else "") + value.lstrip("v")


def read_pin(fmt: str, text: str, key: str | None = None):
    """A pin file's version as a spec, None when it names no version (a
    channel like "stable"), or NOT_PINNED when it doesn't pin this language."""
    if fmt not in FORMATS:
        raise ValueError(f"unknown pin file format {fmt!r} (known: {', '.join(sorted(FORMATS))})")
    return FORMATS[fmt](text, key)
