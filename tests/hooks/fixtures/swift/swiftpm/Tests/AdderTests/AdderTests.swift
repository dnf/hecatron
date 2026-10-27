import XCTest

@testable import Adder

final class AdderTests: XCTestCase {
  func testAdd() {
    XCTAssertEqual(add(2, 3), 5)
  }
}
