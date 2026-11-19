package fr.plys.tv.ui

import org.junit.Assert.assertEquals
import org.junit.Test

class PlaybackPositionTest {
    @Test fun `progress includes server keyframe offset`() {
        assertEquals(120.0, sourcePosition(112.0, 8000, 600.0), 0.001)
        assertEquals(600.0, sourcePosition(112.0, 900000, 600.0), 0.001)
        assertEquals(112.0, sourcePosition(112.0, -1000, 600.0), 0.001)
        assertEquals(120.0, sourcePosition(112.0, 8000, 0.0), 0.001)
    }

    @Test fun `seeking is clamped to the source rather than HLS timeline`() {
        assertEquals(90.0, seekPosition(120.0, -30.0, 600.0), 0.001)
        assertEquals(0.0, seekPosition(5.0, -30.0, 600.0), 0.001)
        assertEquals(599.0, seekPosition(590.0, 30.0, 600.0), 0.001)
        assertEquals(150.0, seekPosition(120.0, 30.0, 0.0), 0.001)
    }

    @Test fun `stream start never asks Media3 for its placeholder default position`() {
        assertEquals(1L, streamStartMs(0.0, 0.0))
        assertEquals(1L, streamStartMs(112.0, 112.0))
        assertEquals(1L, streamStartMs(110.0, 112.0))
        assertEquals(8000L, streamStartMs(120.0, 112.0))
    }

    @Test fun `formats resume times independently of locale`() {
        assertEquals("2:00", clock(120.0))
        assertEquals("1:01:01", clock(3661.0))
        assertEquals("0:00", clock(-5.0))
    }

    @Test fun `seek loading follows the current controls request`() {
        assertEquals(false, playbackControlsVisible(requested = false, loading = true, seekInProgress = true))
        assertEquals(true, playbackControlsVisible(requested = true, loading = true, seekInProgress = true))
        assertEquals(true, playbackControlsVisible(requested = false, loading = true, seekInProgress = false))
        assertEquals(false, playbackControlsVisible(requested = false, loading = false, seekInProgress = false))
    }
}
