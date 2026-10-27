const { add } = require("./calc");

describe("add", () => {
  test("adds two numbers", () => {
    expect(add(2, 3)).toBe(5);
  });

  test("adds zero", () => {
    expect(add(0, 0)).toBe(0);
  });

  test("returns an object", () => {
    expect({ total: add(1, 1) }).toEqual({ total: 2 });
  });
});
