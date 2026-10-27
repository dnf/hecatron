using Xunit;

namespace Add.Tests;

public class AdderTest
{
    [Fact]
    public void AddsTwoNumbers()
    {
        Assert.Equal(5, Adder.Add(2, 3));
    }
}
