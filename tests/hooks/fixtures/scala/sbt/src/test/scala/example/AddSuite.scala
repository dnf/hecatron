package example

class AddSuite extends munit.FunSuite:
  test("adds two numbers") {
    assertEquals(Add.add(2, 3), 5)
  }
