package fr.plys.tv.ui

import fr.plys.tv.data.ExternalSubtitle
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.MediaTrack
import fr.plys.tv.data.PlaybackSelection
import fr.plys.tv.data.Version
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class TrackLanguagesTest {
    private val version = Version(
        "v", "1080p",
        audio = listOf(MediaTrack(1, "eng", default = true), MediaTrack(2, "fre"), MediaTrack(3, "")),
        subtitles = listOf(MediaTrack(4, "fre", forced = true), MediaTrack(5, "fra"), MediaTrack(6, "eng")),
        externalSubtitles = listOf(ExternalSubtitle("x", "pt-BR")),
    )
    private val item = ItemDetail("a", "movie", "Film", versions = listOf(version), children = emptyList())

    @Test fun `every spelling of a language gives the same key`() {
        assertEquals("fr", languageKey("fre"))
        assertEquals("fr", languageKey("fra"))
        assertEquals("fr", languageKey("fr-CA"))
        assertEquals("pt", languageKey("pt-BR"))
        assertNull(languageKey("und"))
        assertNull(languageKey(""))
    }

    @Test fun `a title opens with the first tracks in the preferred languages`() {
        val selection = TrackLanguages(audio = "fr", subtitle = "fr").selection(item)
        assertEquals(PlaybackSelection("v", audioIndex = 2, subtitleIndex = 5), selection)
    }

    @Test fun `without a match the default audio plays without subtitles`() {
        assertEquals(PlaybackSelection("v", audioIndex = 1), TrackLanguages(audio = "de", subtitle = "de").selection(item))
        assertEquals(PlaybackSelection("v", audioIndex = 1), TrackLanguages().selection(item))
    }

    @Test fun `audio and subtitle preferences change apart`() {
        val start = TrackLanguages(audio = "en", subtitle = null)
        val base = PlaybackSelection("v", audioIndex = 1)
        assertEquals(TrackLanguages("fr", null), start.after(item, base, base.copy(audioIndex = 2)))
        assertEquals(TrackLanguages("en", "en"), start.after(item, base, base.copy(subtitleIndex = 6)))
        assertEquals(TrackLanguages("en", "pt"), start.after(item, base, base.copy(externalSubtitleId = "x")))
    }

    @Test fun `subtitles off are remembered, a track without a language is not`() {
        val start = TrackLanguages(audio = "fr", subtitle = "fr")
        val base = PlaybackSelection("v", audioIndex = 2, subtitleIndex = 5)
        assertEquals(TrackLanguages("fr", null), start.after(item, base, base.copy(subtitleIndex = null)))
        assertEquals(start, start.after(item, base, base.copy(audioIndex = 3)))
        assertEquals(start, start.after(item, base, base))
    }
}
