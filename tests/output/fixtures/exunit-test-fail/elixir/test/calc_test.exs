defmodule CalcTest do
  use ExUnit.Case

  test "adds two numbers" do
    assert Calc.add(2, 3) == 5
  end

  test "adds zero" do
    assert Calc.add(0, 0) == 0
  end

  test "rejects nil" do
    Calc.add(nil, 1)
  end
end
