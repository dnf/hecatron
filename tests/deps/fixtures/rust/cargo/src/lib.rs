pub fn render(n: u32) -> String {
    itoa::Buffer::new().format(n).to_owned()
}
