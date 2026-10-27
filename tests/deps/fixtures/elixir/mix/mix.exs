defmodule Fixture.MixProject do
  use Mix.Project

  def project do
    [app: :fixture, version: "0.1.0", elixir: "~> 1.14", deps: deps()]
  end

  defp deps do
    [{:jason, "1.2.2"}]
  end
end
