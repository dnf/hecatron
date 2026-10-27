import "../add" for Add

var sum = Add.add(2, 3)
if (sum != 5) Fiber.abort("expected 2 + 3 = 5, got %(sum)")
System.print("add: 1 check passed")
