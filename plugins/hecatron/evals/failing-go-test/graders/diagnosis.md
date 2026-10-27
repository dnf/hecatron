---
type: llm
weight: 2
---

PASS if the reply says that TestAdd fails, that Add(2, 3) returns 5 while the test expects 6, and that the test's expectation (not Add) is what's wrong.
FAIL if it doesn't name the failing test, says the tests pass, blames Add, or reports that it couldn't run the tests.
