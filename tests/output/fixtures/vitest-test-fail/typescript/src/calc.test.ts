import { describe, expect, it } from "vitest";
import { add } from "./calc";

describe("add", () => {
  it("adds two numbers", () => {
    expect(add(2, 3)).toBe(5);
  });

  it("adds zero", () => {
    expect(add(0, 0)).toBe(0);
  });

  it("returns an object", () => {
    expect({ total: add(1, 1) }).toEqual({ total: 2 });
  });
});
