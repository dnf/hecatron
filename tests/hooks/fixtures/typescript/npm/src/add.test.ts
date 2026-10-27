// node:test without @types/node: declare the two functions used.
declare function require(name: string): any;
const test = require("node:test");
const assert = require("node:assert");
import { add } from "./add";

test("adds", () => {
  assert.strictEqual(add(2, 3), 5);
});
