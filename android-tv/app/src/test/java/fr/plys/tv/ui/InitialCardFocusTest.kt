package fr.plys.tv.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class InitialCardFocusTest {
    @Test
    fun cardLeftFocusedComesFirst() {
        assertEquals("b", initialCardFocus("b", "a", cardsTakeFocus = true))
    }

    @Test
    fun firstCardOtherwise() {
        assertEquals("a", initialCardFocus(null, "a", cardsTakeFocus = true))
    }

    @Test
    fun noCardTakesTheFocusFromTheSearchField() {
        assertNull(initialCardFocus(null, "a", cardsTakeFocus = false))
        assertNull(initialCardFocus("b", "a", cardsTakeFocus = false))
    }
}
