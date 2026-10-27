module Add.Tests.AdderTest

open Xunit
open Add

[<Fact>]
let ``adds two numbers`` () = Assert.Equal(5, Adder.add 2 3)
