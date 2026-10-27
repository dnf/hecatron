-module(add_tests).
-include_lib("eunit/include/eunit.hrl").

add_test() ->
    ?assertEqual(5, add:add(2, 3)).
