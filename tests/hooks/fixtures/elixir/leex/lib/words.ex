defmodule Words do
  @moduledoc "Counts words, with a leex lexer."

  @doc "The number of words in a string."
  def count(text) do
    {:ok, tokens, _} = :words_lexer.string(String.to_charlist(text))
    length(tokens)
  end
end
