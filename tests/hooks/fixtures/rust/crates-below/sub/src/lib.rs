/// Returns a - b.
pub fn sub(a: i64, b: i64) -> i64 {
    a - b
}

#[cfg(test)]
mod tests {
    #[test]
    fn subtracts() {
        assert_eq!(super::sub(5, 3), 2);
    }
}
