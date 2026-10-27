/// Returns a + b.
pub fn add(a: i64, b: i64) -> i64 {
    a + b
}

#[cfg(test)]
mod tests {
    #[test]
    fn adds() {
        assert_eq!(super::add(2, 3), 5);
    }
}
