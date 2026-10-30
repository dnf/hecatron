- **A workspace at the root is its language's component:** a Cargo
  workspace whose members sit in `server/`, `crates/` and `clients/rust/`
  is Rust's at the root, so `hct test rust` runs `cargo test` for every
  member instead of `clients/rust/` alone (#255). The same holds for
  `go.work`, Gradle's `settings.gradle(.kts)`, a Maven `pom.xml` with
  `<modules>`, a `.sln` or `.slnx`, npm, Yarn and pnpm workspaces, and uv
  workspaces, whenever some of the language's packages sit outside the
  directory named for it.

- **A root package beside packages below it is a component too:** a
  `package.json` with dependencies above serverless layers
  (`layers/a/nodejs/`, `layers/b/nodejs/`) gives components `.`,
  `layers/a/nodejs` and `layers/b/nodejs`, so the root's dependencies are
  tested, audited and licensed (#238); a root `go.mod` or Cargo package
  beside one in `clients/go/` or `clients/rust/` does the same. A
  workspaces root and a `package.json` carrying only tooling stay as
  they were. The Node hooks at the root no longer run packages below that
  are components of their own, and a directory several components share
  (a language whose `implPath` doesn't vary by component) runs once.

- **A language at the root claims only its own files:** with a Cargo
  workspace at the root, `hct status` and `--changed` give Rust only Rust's
  changes (`*.rs`, `Cargo.toml`, `Cargo.lock`, `rust-toolchain.toml`...);
  a crash dump or a README beside the crates is another change, not a
  reason to run `hct test rust`. The same holds for a directory of the
  language's that holds other components, for every definition with the
  new `sourceFiles` key (C#, Go, Java, JavaScript, Kotlin, Python, Rust).
  Files inside another git repository or worktree below the root (an
  agent's worktree in `.claude/worktrees/`, a clone) are no longer counted
  as the project's changes at all.
