Imports Xunit

Public Class AdderTest
    <Fact>
    Public Sub AddsTwoNumbers()
        Assert.Equal(5, Adder.Add(2, 3))
    End Sub
End Class
