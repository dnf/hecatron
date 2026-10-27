%% A leex lexer: mix compiles it with leex, which needs parsetools'
%% leexinc.hrl (erlang-dev on Debian).
Definitions.

L = [a-z]
WS = [\s\t\n]

Rules.

{L}+ : {token, {word, TokenLine, TokenChars}}.
{WS}+ : skip_token.

Erlang code.
