- **pnpm and Yarn for every language's hooks:** Corepack's pnpm and Yarn
  shims live in `~/.cache/hecatron/corepack-shims`, which `hct install`
  now creates when a package needs them and which is a static `path`
  entry of JavaScript and TypeScript, so another language's build or
  tests that run `pnpm` (a Rust test in a pnpm monorepo) find it.
