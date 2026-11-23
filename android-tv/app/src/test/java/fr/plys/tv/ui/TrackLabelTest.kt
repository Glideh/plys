package fr.plys.tv.ui

import fr.plys.tv.data.MediaTrack
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.util.Locale

class TrackLabelTest {
    @Test
    fun trackLabelNamesTheLanguageInFullLikeTheWeb() {
        val track = MediaTrack(index = 1, language = "fre", title = "VFF", codec = "eac3", channels = 6)
        assertEquals("Français · EAC3 · 5.1", track.trackLabel(Locale.FRENCH))
        assertEquals("French · EAC3 · 5.1", track.trackLabel(Locale.ENGLISH))
        assertEquals("Anglais · DTS · 5.1", MediaTrack(index = 0, language = "eng", codec = "dts", channels = 6).trackLabel(Locale.FRENCH))
    }

    @Test
    fun trackLabelFallsBackToTitleThenQuestionMark() {
        assertEquals("VFF · AC3 · 5.1", MediaTrack(index = 0, language = "und", title = "VFF", codec = "ac3", channels = 6).trackLabel(Locale.FRENCH))
        assertEquals("? · HDMV_PGS_SUBTITLE", MediaTrack(index = 2, codec = "hdmv_pgs_subtitle").trackLabel(Locale.FRENCH))
    }

    @Test
    fun languageNameUsesTheDeviceLanguage() {
        assertEquals("Allemand", languageName("ger", Locale.FRENCH))
        assertEquals("Japanese", languageName("jpn", Locale.ENGLISH))
        assertEquals("Français (Canada)", languageName("fr-CA", Locale.FRENCH))
    }

    @Test
    fun languageNameKeepsUnknownCodesAndSkipsUndetermined() {
        assertEquals("qaa", languageName("qaa", Locale.FRENCH))
        assertNull(languageName("und", Locale.FRENCH))
        assertNull(languageName(" ", Locale.FRENCH))
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
