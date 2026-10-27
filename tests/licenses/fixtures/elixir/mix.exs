defmodule LicensesFixture.MixProject do
  use Mix.Project

  def project, do: [app: :licenses_fixture, version: "0.1.0", deps: deps()]

  defp deps, do: [{:jason, "1.4.1"}, {:decimal, "2.1.1"}]
end
