- **Tested on 149 open-source projects:** [docs/real-projects.md](docs/real-projects.md)
  shows hecatron run as a newcomer would, on public repositories (polyglot
  ones such as Protocol Buffers, Thrift, Avro and gRPC first, then
  well-known projects in each language and some large monorepos): each
  cloned on a fresh Ubuntu machine and run through `hct init`, `install`,
  `doctor` and `test` with no configuration, every month. It lists the
  results per project and per language, why languages fail, and a badge
  for each project that follows its latest result; the data is in
  `tests/real-projects/` (`projects.json`, `results.json`,
  `results-by-language.json`, described by `schema.json`) (#163).
