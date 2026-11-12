package fr.plys.tv.ui

import org.junit.Assert.assertEquals
import org.junit.Test
import java.util.Locale

class SubtitleLanguageTest {
    @Test fun `default subtitle language follows the device locale`() {
        assertEquals("fr", defaultSubtitleLanguage(Locale.FRANCE))
        assertEquals("pt-BR", defaultSubtitleLanguage(Locale.forLanguageTag("pt-BR")))
        assertEquals("pt-PT", defaultSubtitleLanguage(Locale.forLanguageTag("pt-AO")))
        assertEquals("zh-TW", defaultSubtitleLanguage(Locale.TAIWAN))
        assertEquals("en", defaultSubtitleLanguage(Locale.forLanguageTag("sw-KE")))
    }
}
