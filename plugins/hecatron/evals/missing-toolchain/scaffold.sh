#!/usr/bin/env bash
# A git repository with a small Mix project (run where Elixir isn't installed).
set -euo pipefail
mkdir -p lib test
printf 'defmodule Calc.MixProject do\n  use Mix.Project\n\n  def project, do: [app: :calc, version: "0.1.0", elixir: "~> 1.14", deps: []]\nend\n' > mix.exs
printf 'defmodule Calc do\n  def add(a, b), do: a + b\nend\n' > lib/calc.ex
printf 'ExUnit.start()\n' > test/test_helper.exs
printf 'defmodule CalcTest do\n  use ExUnit.Case\n  test "add" do\n    assert Calc.add(2, 3) == 5\n  end\nend\n' > test/calc_test.exs
git init -q
git add -A
git -c user.email=eval@example.com -c user.name=eval commit -qm "Add calc"
