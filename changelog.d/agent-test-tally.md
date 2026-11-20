- **`hct test` says how many tests ran:** agent output gives each passed
  job a `PASS` line with its time and log, and a test job's `PASS` or
  `FAIL` line ends with each test runner's own result line (pytest's
  `246 passed, 15 skipped in 0.14s`, RSpec's `3 examples, 2 failures`,
  cargo's `test result:`, a count of Go's packages, and more); JSON output
  has them as `tally`. In the agent comparison, agents asked for the counts
  read the logs or ran the tests again to find them.
