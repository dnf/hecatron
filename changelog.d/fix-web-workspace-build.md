- **Node.js workspaces built before their tests:** when a package in the
  workspace a test runs in needs building (a native addon or entry point
  not built yet), `hct test typescript` and `hct test javascript` build
  the workspace first (`pnpm -r run build`, `npm run build --workspaces`,
  Yarn's `workspaces foreach --topological run build`), so a root `test`
  script that runs every package's tests, or a package importing another
  one's `dist/`, finds them built.
