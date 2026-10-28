- **Rust's system dependencies:** `hct install rust` reads every Cargo.lock
  in the project outside the trees detection skips (examples, tests,
  vendored code...), not only those of the Rust components, so a crate
  another language's build compiles (a Java binding's Maven build running
  cargo) gets what it needs too, such as `protoc`.
