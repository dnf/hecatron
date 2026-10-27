# frozen_string_literal: true

require 'minitest/autorun'
require 'add'

class TestAdd < Minitest::Test
  def test_adds_two_numbers
    assert_equal 5, Add.add(2, 3)
  end
end
