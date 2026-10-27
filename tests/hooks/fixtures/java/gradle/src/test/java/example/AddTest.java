package example;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

class AddTest {
    @Test
    void adds() {
        assertEquals(5, Add.add(2, 3));
    }
}
