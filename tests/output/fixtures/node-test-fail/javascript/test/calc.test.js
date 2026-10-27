const test = require("node:test");
const assert = require("node:assert/strict");
const { add } = require("../calc");

test("adds two numbers", () => {
  assert.equal(add(2, 3), 5);
});

test("adds zero", () => {
  assert.equal(add(0, 0), 0);
});

test("returns an object", () => {
  assert.deepEqual({ total: add(1, 1) }, { total: 2 });
});
