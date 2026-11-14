package fr.plys.tv.ui

import fr.plys.tv.data.MediaTrack
import org.junit.Assert.assertEquals
import org.junit.Test

class TrackLabelTest {
    @Test
    fun shortLabelKeepsLanguageAndTitleOnly() {
        val track = MediaTrack(index = 1, language = "fre", title = "VFF", codec = "eac3", channels = 6)
        assertEquals("fre · VFF · EAC3 · 5.1", track.trackLabel())
        assertEquals("fre · VFF", track.shortLabel())
    }

    @Test
    fun shortLabelUsesLanguageAlone() {
        assertEquals("eng", MediaTrack(index = 0, language = "eng", codec = "aac", channels = 2).shortLabel())
    }

    @Test
    fun shortLabelFallsBackToFullLabelWhenUnnamed() {
        assertEquals("#3 · AAC · 2.0", MediaTrack(index = 2, codec = "aac", channels = 2).shortLabel())
    }
}
