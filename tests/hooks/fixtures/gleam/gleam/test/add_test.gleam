import add
import gleeunit

pub fn main() {
  gleeunit.main()
}

pub fn add_test() {
  assert add.add(2, 3) == 5
}
