//! A benchmark that runs on stable Rust: a plain program (harness = false).
use std::hint::black_box;
use std::time::Instant;

fn main() {
    let start = Instant::now();
    let mut total = 0;
    for i in 0..1_000_000 {
        total = add::add(black_box(total), black_box(i));
    }
    println!("add: 1000000 calls in {:?} ({total})", start.elapsed());
}
