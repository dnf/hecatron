- **Rust tests:** `hct test rust` runs `cargo test` rather than `cargo test
  --all-targets`: Cargo's default targets, now with doctests, and without
  running benchmarks as tests (a nightly-only benchmark broke stable builds,
  and others fail or run long in test mode).
