import assert from "node:assert/strict";
import { test } from "node:test";

import { add } from "add";

test("adds two numbers", () => {
  assert.equal(add(2, 3), 5);
});
