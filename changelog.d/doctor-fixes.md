- **A missing lint tool no longer fails `hct doctor`:** golangci-lint (Go),
  luacheck (Lua) and ShellCheck (Bash) not on PATH is a note under the
  language ("lint tool golangci-lint not found: hct lint go needs it"), not
  a FAIL, and `doctor --fix` no longer installs one by itself; `hct
  install` still does, and `hct lint` still needs it. A repository that
  lints its own way (a project `lint.sh`) is healthy without it. Go's
  default linter is unchanged (#254).
- **The .NET runtime check asks only for runtimes something runs on:** a
  test project, an executable or a web or worker app targeting `net8.0`
  needs the .NET 8 runtime; a class library multi-targeting `net8.0` only
  builds, with the targeting pack the SDK restores, and no longer makes the
  doctor fail or the installer install that runtime. Frameworks come from
  the project file, else the nearest `Directory.Build.props` (#256).
- **`package.json`'s `engines.node` is read as a range:** a component on
  the default Node.js with none of the default's line installed runs the
  newest installed Node.js in the range (`>=22`, `^20 || ^22`, `20 - 22`),
  and the doctor counts it as installed and says so, instead of failing
  "typescript =24.22.1 not installed (from default)". A definition lists
  such files in `rangeFiles` (format `npm-engines`) (#257).
- **`hct status` agrees with the doctor about versions:** a toolchain on
  PATH without the version a component needs shows as "installed, but not
  24.22.1 (web)" (`versionMissing` in JSON), with `hct install` suggested.
- **In a checkout, binary-only commands say to run `hct self build`:**
  `hct status`, `sync`, `advisor`, `self update` and the rest, without the
  binary, add "in this checkout, run hct self build" (#258).
