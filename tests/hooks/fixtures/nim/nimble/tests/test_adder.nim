import std/unittest
import adder

suite "adder":
  test "adds two numbers":
    check add(2, 3) == 5
