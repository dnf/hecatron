- **`hct go [lang...]`:** getting a project's tests running in one
  command: each language and its version in one line, `hct install` with
  its output in a log and one line for it, then `hct test` with each test
  runner's counts. An agent's first command in a repository, in place of
  `hct status`, `hct install` and `hct test` one turn at a time; the Agent
  Skill now starts with it, and `hct status` suggests it in place of an
  `hct install` line per language. It installs toolchains, as
  `hct install` does. Needs the binary.
