rockspec_format = "3.0"
package = "add"
version = "0.1.0-1"
source = {
   url = "git+https://example.com/add.git",
}
description = {
   summary = "Adds two numbers.",
   license = "MIT",
}
dependencies = {
   "lua >= 5.1",
}
build = {
   type = "builtin",
   modules = {
      add = "src/add.lua",
   },
}
