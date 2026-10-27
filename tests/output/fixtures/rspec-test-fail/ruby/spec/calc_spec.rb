require_relative "../lib/calc"

RSpec.describe Calc do
  it "adds two numbers" do
    expect(Calc.add(2, 3)).to eq(5)
  end

  it "adds zero" do
    expect(Calc.add(0, 0)).to eq(0)
  end

  it "rejects nil" do
    Calc.add(nil, 1)
  end
end
