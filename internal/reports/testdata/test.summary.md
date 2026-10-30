### hct test: 1 passed, 3 failed, 3 skipped (14s)

| Language | Component | Status | Time |
| --- | --- | --- | --- |
| go | age | **failed** | 7.0s |
| go | date | passed | 5.0s |
| python | . | **failed** | 1.5s |
| rust |  | not installed |  |
| javascript | . | skipped: package.json has no test script | 0.2s |
| elm |  | skipped: no test runner for elm | 0.1s |
| docs |  | passed | 0.3s |
| flaky |  | **failed** | 0.1s |

#### FAIL go (age/go/): exit 1, 12s

Log: `testdata/logs/test-go.log`

```text
--- FAIL: TestAdd (0.00s)
calc_test.go:7: Add(2, 3) = -1, want 5
```

#### FAIL python (python/): exit 1, 1.5s

Log: `testdata/logs/none.log`

#### FAIL flaky: exit 4, 0.1s

Log: `testdata/logs/test-flaky.log`

````text
flaky failed: | pipes & <tags> ```fence```
````

Rerun: `hct test go python flaky`

