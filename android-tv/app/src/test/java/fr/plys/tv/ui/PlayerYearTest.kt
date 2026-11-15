package fr.plys.tv.ui

import fr.plys.tv.data.ItemDetail
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class PlayerYearTest {
    private fun item(kind: String, year: Int?) = ItemDetail("a", kind, "Title", year = year, versions = emptyList(), children = emptyList())

    @Test fun `films and videos show their year`() {
        assertEquals(1999, playerYear(item("movie", 1999)))
        assertEquals(2021, playerYear(item("video", 2021)))
    }

    @Test fun `episodes and unknown years show none`() {
        assertNull(playerYear(item("episode", 2010)))
        assertNull(playerYear(item("movie", null)))
        assertNull(playerYear(item("movie", 0)))
    }
}
