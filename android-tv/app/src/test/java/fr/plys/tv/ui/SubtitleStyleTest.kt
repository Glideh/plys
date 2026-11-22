package fr.plys.tv.ui

import org.junit.Assert.assertEquals
import org.junit.Test

class SubtitleStyleTest {
    @Test fun `stored keys are read back`() {
        assertEquals(SubtitleStyle(SubtitleSize.EXTRA_LARGE, SubtitleBackground.BOX), subtitleStyleOf("xlarge", "box"))
        assertEquals(SubtitleStyle(SubtitleSize.SMALL, SubtitleBackground.OUTLINE), subtitleStyleOf("small", "outline"))
    }

    @Test fun `missing or unknown keys fall back to the default`() {
        assertEquals(SubtitleStyle(), subtitleStyleOf(null, null))
        assertEquals(SubtitleStyle(), subtitleStyleOf("huge", "blur"))
    }

    @Test fun `hidden controls keep the usual margin`() {
        assertEquals(60, subtitleBottomPadding(screenHeight = 1000, gapBelow = 0, controlsHeight = null))
        assertEquals(60, subtitleBottomPadding(screenHeight = 1000, gapBelow = 130, controlsHeight = null))
    }

    @Test fun `shown controls lift subtitles just above them`() {
        // A full screen picture: the whole controls' height.
        assertEquals(400, subtitleBottomPadding(screenHeight = 1000, gapBelow = 0, controlsHeight = 400))
        // A letterboxed picture: the black bar under it already covers part of the controls.
        assertEquals(270, subtitleBottomPadding(screenHeight = 1000, gapBelow = 130, controlsHeight = 400))
        // Controls shorter than the bar: subtitles keep their margin.
        assertEquals(60, subtitleBottomPadding(screenHeight = 1000, gapBelow = 380, controlsHeight = 400))
    }
}
