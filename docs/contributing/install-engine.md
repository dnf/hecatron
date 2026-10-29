# The install engine: installs as data (#77)

Status: **stage 1 implemented** (the executor, dispatch and the parity
harness, proven on eleven languages whose installers only install system
packages). The format and its staging come from a survey of all the
installers; the rest of the definitions are converted in later batches.

## Decided by the owner (2026-10-28)

1. **Install data lives in `languages/<lang>/install.json`**, a file of its
   own beside `config.json`, not in it. A project's
   `.hecatron/languages/<lang>/install.json` replaces the built-in one
   whole, as an `install.sh` override does, rather than merging key by key
   as `config.json` does ([resolving a language's files](../guide/configuration.md#resolving-a-languages-files)): a project replacing an install wants to
   replace all of it. Versions stay in `config.json` (`version`, `track`,
   `pinFiles`, `path`, `env`), where resolution ([versions](../guide/versions.md)) reads them; an
   install never states its toolchain's version. The prototype's
   `"install"` key moved out of Go's, Rust's, Python's and Java's
   `config.json` into their `install.json` (Elixir's stays until its
   installer's rewrite, #164, settles).
2. **macOS uses the upstream release download where there is one**, so
   macOS and Linux install the same pinned version of a language
   downloaded as an archive; Homebrew stays for system packages (the
   languages that come from apt on Linux). Done for the release
   downloads converted so far (Gleam, Zig).
3. **PATH: links to each language's default version in one hecatron bin
   directory**, which the `~/.hecatron/env` the top-level `install.sh`
   already writes puts on PATH; no shim. Hecatron's own commands put each
   component's version on PATH themselves ([versions](../guide/versions.md)), so the links only serve
   interactive shells. The directory is `~/.local/share/hecatron/bin`
   (under `$XDG_DATA_HOME` when set), which `~/.hecatron/env` puts on PATH
   after hecatron's own `bin/`.

## The format

`install.json` has a `schema` (1), a `toolchain` (what
provides the language: `"versions"`, `"system"` for the package manager's
version, `"single"` or `"managed"` for versions hecatron installs, and one
install kind: `packages` per package manager, or `archive`, a release
download; later `vendor` and `via`), then optional lists, each entry with
conditions:

- `packages`: more system packages, with `when` (install only if it holds)
  and `unless` (already satisfied: D's `build-essential` unless there's a
  `cc`);
- `tools`: programs the hooks need (ShellCheck, Bats, zef), each with a
  `name`, its `packages`, a `when` whose `because` is the doctor's reason
  ("bats not installed (a component has .bats tests)") and a `check`;
- `steps`: commands run after the packages (`raco pkg install
  compiler-lib`), each with a `check` and the `problem` the doctor reports
  while it fails; a `download` (`url` and its mandatory `sha256`, cached
  and verified as archives are) the run names as `{file}`, `{data}` for
  hecatron's data directory, and `creates`, a directory inside it removed
  before the step runs again and by uninstall (Common Lisp's Quicklisp,
  installed with its quickstart `quicklisp.lisp` into
  `~/.local/share/hecatron/quicklisp`, no root);
- `aptSources`: a signed apt repository for one distribution (`distro`,
  /etc/os-release's `ID`) whose own packages are too old (SWI-Prolog's PPA
  on Ubuntu, whose 9.0 lsp_server can't use): `uri`, `suite` (default
  `{codename}`), `components`, and the `key` URL and its `fingerprint`,
  checked in Go (the primary key's, and only one key) before the key goes
  in `/etc/apt/keyrings/hecatron-<name>.asc` and the repository in
  `/etc/apt/sources.list.d/hecatron-<name>.sources`, signed by it alone.
  Adding one refreshes the package lists, upgrades the `packages` it
  provides that apt installed from the distribution (not receipted: they
  weren't hecatron's), and first removes the installed `replaces`, the
  distribution's packages whose files its packages take over without
  saying so (Ubuntu's `swi-prolog-core`). A release the repository has no
  suite for keeps the distribution's packages. The doctor reports a source
  not added; uninstall removes the two files;
- `uninstall.keep`: packages uninstall never removes, with `because`
  (COBOL's cmake, which C, C++ and Fortran install with their scripts).

Conditions are the hooks' tier-1 vocabulary (`file`, `missing`, `all`,
`any`, `not`) plus `glob` (a file so named anywhere below a component's
directory), `command` (on the language's PATH), `run` (exits 0) and
`platform`; file tests hold when any component's directory satisfies them.
Anything needing a loop, string parsing or a second command's output is a
hook or family code, not new syntax. Later stages add `vendor` and `via`
toolchains (`"managed"` release downloads are in, below), shared tools in
`languages/_tools.json`,
`installed` (in place of `installed.sh`), `requires` and `installOf`,
`hook`, `deps` and `legacy` (old locations to clean up).

## Which runs

`hct install`, `uninstall` and `doctor` (and `--fix`)
choose per language: the project's own `install.sh` (or `uninstall.sh`,
`doctor.sh`) always wins; `HECATRON_INSTALLER=script` forces the scripts;
otherwise an `install.json` the binary can carry out on this platform
(a known schema, only keys and kinds it runs, packages for this machine's
package manager: apt on Debian and Ubuntu, Homebrew on macOS); otherwise
`install.sh`. A key the binary doesn't know declines the definition rather
than being skipped, so a definition written for a newer binary still
installs, through its script. Without the binary, `bin/hecatron` runs the
scripts, so a converted language keeps its `install.sh`, `uninstall.sh`
and `doctor.sh` until 1.0 (#79).

Other installers still run some languages' `install.sh` directly
(Elixir's and Gleam's scripts, and Gleam's data as a step, run Erlang's). A
converted language that something calls starts its `install.sh` with
`install_from_data <lang>` (`_common.sh`): given the binary
(`HECATRON_BINARY`, set for every installer the binary runs) and not
forced to the scripts, it runs `hecatron _install <lang>`, which
installs from the data or exits 125 when it wouldn't (a project's own
`install.sh`, a platform the data doesn't cover), and the script then
carries on itself. `_install` never runs a script, so the two can't loop.
Declaring such dependencies in the data (`requires`) comes later; callers
don't depend on it.

## The executor (`internal/install`)

- **Plan.** Every action of every language being installed, each with its
  check: a toolchain's or a `packages` entry's packages are installed
  (`dpkg-query`, `brew list`), or its `unless` holds; a tool's or step's
  `check` holds. An action whose `when` fails, or with no package for this
  manager, is skipped. `hct install --plan [lang...]` prints the plan, the
  languages left to their scripts and why, and the commands it would run,
  changing nothing.
- **Apply.** The missing packages of every language go to one package
  manager call (`apt-get install`, after `apt-get update` when the lists
  are over an hour old as `apt_install` does; `brew install`), with `sudo`
  unless running as root; if that fails, each language's alone, so one bad
  package fails only its language. Then each language's steps whose checks
  fail, each checked again after it runs, then every check again: an action
  still failing fails its language and is named. A rerun finds every check
  passing and runs nothing.
- **Doctor.** A language installed from data gets the doctor's checks from
  it, in place of `doctor.sh`: after the `dependency` and `lintDependency`
  checks and the version notes, each tool and step that applies and whose
  check fails (a tool checked as the `dependency` or `lintDependency`
  already is isn't reported twice). `--fix` applies the language's plan,
  which runs only what fails.
- **Receipts.** `~/.local/share/hecatron/receipts/<lang>.json`
  (`$XDG_DATA_HOME/hecatron/receipts` when set) records the packages the
  executor installed, only those absent before, so `python3` or a system
  Bash is never hecatron's to remove, and the steps it ran.
- **Uninstall** reads the receipt: it removes the packages it lists that
  are still installed, except those the definition keeps, another
  language's receipt lists, or another definition names, and says which it
  keeps and why; then it removes the receipt. With no receipt (installed by
  the scripts, or before hecatron) it removes nothing and says so;
  `HECATRON_INSTALLER=script` runs the old `uninstall.sh`.

## Converted (stage 1)

Bash, COBOL, D, Common Lisp, Octave, Pascal,
Prolog, Racket, Raku and Tcl. Elixir and Erlang, which the survey listed as
packages only, are no longer: Elixir's installer installs builds.hex.pm's
releases side by side (#164), and Erlang's builds.hex.pm's OTP build on
Ubuntu, whose own is too old for current libraries (#206), falling back to
the distribution's on Debian, which builds.hex.pm doesn't build for. The
OTP install is shared (`otp_install_ubuntu` in `_common.sh`): Erlang's goes
in the toolchains directory, Elixir's in `/opt/erlang`.

## Release downloads (stage 2)

A `toolchain.archive` is a release
download, installed `"versions": "single"` (the definition's default from
`config.json`) or `"managed"`: the default and, side by side, each version
the project's components resolve to ([versions](../guide/versions.md)) that `installed.sh` doesn't
list, a prefix or line as the newest release in the definition's
`releases.json` it matches, so planning stays offline (a version with none
fails that language's install and says so). Each version is its own
toolchain action, checked in its own home; the links point at the
default.

- `url` is a template (`{version}`, `{os}`, `{arch}`, `{ext}`, with `os`,
  `arch` and `ext` mapping hecatron's names to the publisher's); a
  platform with no mapping isn't installed from data.
- **A checksum is mandatory**, from one of: `sha256` inline by platform
  (`"linux-x86_64": "…"`), `checksum`, a URL template serving it (a
  sidecar such as Gleam's `{url}.sha256`: its first word), or
  `checksumIndex`, a JSON index and the keys leading to it (Zig's
  `index.json`, `["{version}", "{arch}-{os}", "shasum"]`). A definition
  with none for the platform is declined. An index's `urlPath` leads to
  the download's own URL there, which replaces the template: Zig's names
  changed in 0.14.1 (`zig-linux-x86_64-0.14.0` before), and its index
  lists them all.
- `check` is required: what says the unpacked toolchain works
  (`{"run": ["{bin}/zig", "version"]}`, with `{home}` and `{bin}`).
- The download goes to `~/.cache/hecatron/downloads/<sha256>`, verified
  before it is kept (a cached file is checked again before reuse); it is
  unpacked (`tar.gz` and `zip` in Go, `tar.xz` with the system's `tar`,
  `strip` leading components) beside its home and renamed into place, so
  an interrupted install never leaves a half-unpacked home and a broken
  one is replaced whole. The home is
  `~/.local/share/hecatron/toolchains/<lang>/<version>`: no root.
- `links` names the commands linked into the links directory (decision 3);
  a link is replaced, a file that isn't one never is.
- The doctor reports a home that is there but fails its check ("doesn't
  work"), and the usual version check (`installed.sh`) a missing one;
  `--fix` reinstalls from the downloads cache when it can.
- The receipt records the homes and links, and uninstall removes them
  (only ever under the toolchains and links directories).

Converted: **Gleam** (GitHub release, sidecar checksum; Erlang/OTP from
Erlang's `install.sh`, a step, unless the `erl` on PATH is OTP 26 or newer) and **Zig** (ziglang.org,
checksum and URL from its index; it installed the latest release before,
and now the version `config.json` pins and, `"managed"`, each line a
component's `build.zig.zon` names). Both gain `version`, `track` and
`path: ["{home}"]` in `config.json` and an `installed.sh` (the toolchains
directory, then one on PATH from elsewhere), and their scripts install to
the same place, on macOS too, checked the same way, so the two ways agree
for the parity harness and for `bin/hecatron` without the binary. Not
converted: Goose, whose `goose version` prints no version, so a pin can't
be checked on a machine set up by its script; the "latest" languages
(Crystal, Haxe, Julia, Nim, Odin, PowerShell, Swift), which need a pinned
version and a source to update it first.

## Parity

`tests/docker/parity.sh <image> <lang>...` installs each
language in two fresh containers with the binary built from the checkout,
once forced to the scripts and once from its data; each must pass
`tests/install/check.sh` (install, doctor from a login shell, rerun, smoke),
and both must end with the same installed packages and versions, the same
commands on PATH and the same doctor report. Project files in
`tests/docker/parity/<lang>/` (a `.bats` test, a `META6.json`) exercise
what installs only for some files.

## Open

pinning the twelve languages that install "latest" today, with
`update-versions` sources; version managers' homes (`~/.rustup`, `~/.ghcup`,
`~/.opam`, or under hecatron's toolchains directory); refusing to remove
packages present before hecatron outright (receipts already never list
them); Perl modules and `npm -g` without sudo; a pure-Go xz reader or the
system `tar`; dnf packages, which nothing tests yet; Windows link
directories (hard links or `.cmd` shims).
