- **`hct exec --output agent`:** runs one command (one crate's or package's
  tests, say) as a run command runs a job for an agent: its output in
  `<logs>/exec/<CMD>.log`, one line when it passes, and when it fails the
  log, its error lines from the language's error patterns and the command
  that reruns it (`HECATRON_RERUN`, when a script running `hct exec` for
  its users sets it). The exit status is still the command's.
- **`"wrapper"` in config.json:** a command every job runs under (each
  hook, project task and lint's shell check, and `hct exec`'s command),
  given the job's command as its arguments: for what every build on a
  shared machine needs, such as a lower priority, a per-checkout build
  directory or a shared compiler cache. Binary only; without the binary
  the run commands refuse to run rather than run jobs without it.
- **A `go test -v` failure's error lines:** the lines of the tests that
  passed or were skipped are left out of the excerpt (a definition's new
  `"errorPassedBlock"`), so ten skipped tests' `x_test.go:12: needs a
  server` lines no longer crowd out the failure.
