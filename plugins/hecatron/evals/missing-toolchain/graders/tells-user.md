---
type: llm
weight: 2
---

PASS if the reply says the tests couldn't run because Elixir (or Erlang/Mix) isn't installed, and tells the user how to install it or offers to (for example `hct install elixir`), leaving the decision to them.
FAIL if it claims the tests ran or passed, or doesn't say what's missing and how to get it.
