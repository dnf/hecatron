- **`--plan` on the run commands:** `hct test --changed --plan` (and
  `lint`, `build`, `retest` and the others, with or without `--changed`)
  prints what the run would do and runs nothing: the base, every changed
  file with what it selects and why (its directory's language and
  component, a build file above languages' directories, hooks, the
  configuration, a task's `taskPaths`, nothing, or inside another git
  repository), the jobs with their components' directories, and the jobs
  skipped up front (disabled, not installed, no hook). Exit 0. Agent
  output gives a line per file, job and skip; `--output json` gives
  `base`, `files`, `jobs` and `skipped` (#263). The bash runner says
  `--plan` and `--changed` need the binary instead of taking them for
  targets.
