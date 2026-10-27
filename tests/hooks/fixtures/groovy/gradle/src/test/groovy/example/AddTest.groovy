package example

import org.junit.jupiter.api.Test

import static org.junit.jupiter.api.Assertions.assertEquals

class AddTest {
    @Test
    void addsTwoNumbers() {
        assertEquals(5, Add.add(2, 3))
    }
}
