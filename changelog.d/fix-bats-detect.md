- **Bash is detected from a Bats suite:** a definition's `detectTests`
  (Bash's `*.bats`) count in `test/` and `tests/` for a language not
  otherwise found, placing it at the directory tested, so bats-core and
  Bash-it are Bash projects. Bash's test hook leaves out `.bats` files
  under a `fixtures/` directory, which are test data.
