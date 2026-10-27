defmodule WordsTest do
  use ExUnit.Case

  test "counts words" do
    assert Words.count("two words") == 2
  end
end
