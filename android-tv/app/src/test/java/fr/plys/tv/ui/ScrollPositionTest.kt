package fr.plys.tv.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ScrollPositionTest {
    @Test
    fun positionIsOneBased() {
        assertEquals(1, scrollPosition(0, 9394))
        assertEquals(42, scrollPosition(41, 9394))
        assertEquals(9394, scrollPosition(9393, 9394))
    }

    @Test
    fun noPositionBeforeFocusOrForAnEmptyCatalog() {
        assertNull(scrollPosition(null, 9394))
        assertNull(scrollPosition(3, 0))
        assertNull(scrollPosition(-1, 10))
    }

    @Test
    fun positionNeverExceedsTotal() {
        assertEquals(10, scrollPosition(15, 10))
    }
}
