package fr.plys.tv.ui

import android.view.KeyEvent
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PlayKeyTest {
    @Test fun `play and play-pause keys play`() {
        assertTrue(isPlayKey(KeyEvent.KEYCODE_MEDIA_PLAY))
        assertTrue(isPlayKey(KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE))
    }

    @Test fun `other keys do not`() {
        assertFalse(isPlayKey(KeyEvent.KEYCODE_MEDIA_PAUSE))
        assertFalse(isPlayKey(KeyEvent.KEYCODE_DPAD_CENTER))
        assertFalse(isPlayKey(KeyEvent.KEYCODE_ENTER))
    }
}
