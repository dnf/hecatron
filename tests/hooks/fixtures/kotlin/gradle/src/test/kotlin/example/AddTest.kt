package example

import kotlin.test.Test
import kotlin.test.assertEquals

class AddTest {
    @Test
    fun addsTwoNumbers() {
        assertEquals(5, add(2, 3))
    }
}
