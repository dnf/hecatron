module AddTest exposing (suite)

import Add exposing (add)
import Expect
import Test exposing (Test, test)


suite : Test
suite =
    test "adds two numbers" <|
        \_ -> Expect.equal 5 (add 2 3)
