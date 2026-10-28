- **Detection:** a language whose build files are all more than three
  levels below the directory the layout gives it (antlr4's Go module in
  `runtime/Go/antlr/v4`, jsii's in `packages/@jsii/...`) gets its own
  `implPaths` entry instead, since the hooks look no deeper and failed with
  "no go.mod here, below, or above it"; files under `vendored/` no longer
  make a language.
