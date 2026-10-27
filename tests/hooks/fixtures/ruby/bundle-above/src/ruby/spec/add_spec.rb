# frozen_string_literal: true

require 'add'

RSpec.describe Add do
  it 'adds two numbers' do
    expect(described_class.add(2, 3)).to eq(5)
  end
end
