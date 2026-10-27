package calc

import kotlin.test.Test
import kotlin.test.assertEquals

class CalcTest {
    @Test
    fun adds() {
        assertEquals(5, add(2, 3))
    }
}
