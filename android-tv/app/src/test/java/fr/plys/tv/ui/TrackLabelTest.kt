package fr.plys.tv.ui

import fr.plys.tv.data.MediaTrack
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class TrackLabelTest {
    @Test
    fun shortLabelKeepsShortLanguageAndTitleOnly() {
        val track = MediaTrack(index = 1, language = "fre", title = "VFF", codec = "eac3", channels = 6)
        assertEquals("fre · VFF · EAC3 · 5.1", track.trackLabel())
        assertEquals("FR · VFF", track.shortLabel())
    }

    @Test
    fun shortLabelUsesLanguageAlone() {
        assertEquals("EN", MediaTrack(index = 0, language = "eng", codec = "aac", channels = 2).shortLabel())
    }

    @Test
    fun shortLabelUsesTitleAloneWhenLanguageUndetermined() {
        assertEquals("Commentary", MediaTrack(index = 0, language = "und", title = "Commentary", codec = "aac", channels = 2).shortLabel())
    }

    @Test
    fun shortLabelFallsBackToFullLabelWhenUnnamed() {
        assertEquals("#3 · AAC · 2.0", MediaTrack(index = 2, codec = "aac", channels = 2).shortLabel())
    }

    @Test
    fun shortLanguageUsesTwoLetterCodes() {
        assertEquals("FR", shortLanguage("fre"))
        assertEquals("FR", shortLanguage("fra"))
        assertEquals("DE", shortLanguage("ger"))
        assertEquals("DE", shortLanguage("deu"))
        assertEquals("JA", shortLanguage("jpn"))
        assertEquals("EN", shortLanguage("en"))
    }

    @Test
    fun shortLanguageKeepsRegionInLowerCase() {
        assertEquals("FR-ca", shortLanguage("fr-CA"))
        assertEquals("PT-br", shortLanguage("pt_BR"))
        assertEquals("FR-ca", shortLanguage("fre-CA"))
    }

    @Test
    fun shortLanguageKeepsUnknownCodesAndSkipsUndetermined() {
        assertEquals("QAA", shortLanguage("qaa"))
        assertNull(shortLanguage("und"))
        assertNull(shortLanguage(" "))
    }
}
