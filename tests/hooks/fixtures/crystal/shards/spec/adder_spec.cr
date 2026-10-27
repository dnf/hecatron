require "spec"
require "../src/adder"

describe Adder do
  it "adds two numbers" do
    Adder.add(2, 3).should eq(5)
  end
end
