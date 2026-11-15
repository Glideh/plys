package fr.plys.tv.ui

import android.view.KeyEvent
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.type

/** The remote's play key: a dedicated play key, or the single play/pause key most remotes have. */
fun isPlayKey(keyCode: Int): Boolean = keyCode == KeyEvent.KEYCODE_MEDIA_PLAY || keyCode == KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE

/**
 * Starts [play] when the play key is pressed while the focus is on this element or inside it. The event bubbles up
 * from the focused element, so an inner handler (an episode tile on a detail page) wins over an outer one (the page).
 * Both the press and the release are consumed so the key never reaches anything else; a held key plays only once.
 */
fun Modifier.onPlayKey(play: (() -> Unit)?): Modifier = if (play == null) this else onKeyEvent { event ->
    if (!isPlayKey(event.nativeKeyEvent.keyCode)) return@onKeyEvent false
    if (event.type == KeyEventType.KeyDown && event.nativeKeyEvent.repeatCount == 0) play()
    true
}
