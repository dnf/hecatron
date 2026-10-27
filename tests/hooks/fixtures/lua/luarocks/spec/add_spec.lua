local add = require("add")

describe("add", function()
   it("adds two numbers", function()
      assert.are.equal(5, add.add(2, 3))
   end)
end)
