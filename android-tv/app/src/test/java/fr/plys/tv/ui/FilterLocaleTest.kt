package fr.plys.tv.ui

import org.junit.Assert.assertEquals
import org.junit.Test

class FilterLocaleTest {
    @Test fun `device language maps to a supported filter locale`() {
        assertEquals("fr", filterLocale("fr"))
        assertEquals("pt-BR", filterLocale("pt"))
        assertEquals("en", filterLocale("en"))
        assertEquals("en", filterLocale("de"))
    }
}
