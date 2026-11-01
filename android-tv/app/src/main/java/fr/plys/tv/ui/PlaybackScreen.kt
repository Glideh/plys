package fr.plys.tv.ui

import android.view.KeyEvent
import android.view.WindowManager
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.media3.ui.PlayerView
import androidx.tv.material3.Button
import androidx.tv.material3.Text
import fr.plys.tv.MainActivity
import fr.plys.tv.PlysApplication
import fr.plys.tv.R
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.PlysApi
import kotlinx.coroutines.delay

@Composable
@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
fun PlaybackScreen(item: ItemDetail, initialPosition: Double, api: PlysApi, onClosed: () -> Unit) {
    val context = LocalContext.current
    val activity = context as MainActivity
    val model = remember(item.id) {
        PlayerController(context.applicationContext as PlysApplication, api, item, initialPosition)
    }
    DisposableEffect(model) { onDispose { model.dispose() } }
    val state by model.state.collectAsStateWithLifecycle()
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    DisposableEffect(model, lifecycle) {
        val observer = LifecycleEventObserver { _, event ->
            when (event) {
                Lifecycle.Event.ON_START -> model.start()
                Lifecycle.Event.ON_STOP -> model.background()
                else -> Unit
            }
        }
        lifecycle.addObserver(observer)
        if (lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)) model.start()
        activity.window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        onDispose {
            lifecycle.removeObserver(observer)
            activity.window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        }
    }
    var showControls by remember { mutableStateOf(true) }
    var interaction by remember { mutableStateOf(0) }
    val rootFocus = remember { FocusRequester() }
    val playFocus = remember { FocusRequester() }
    val visible = showControls || state.paused || state.loading || state.error != null || state.ended
    LaunchedEffect(interaction, state.paused, state.loading, state.error, state.ended) {
        if (!state.paused && !state.loading && state.error == null && !state.ended) {
            delay(5000)
            showControls = false
        }
    }
    LaunchedEffect(visible, state.loading, state.ended, state.error) {
        androidx.compose.runtime.withFrameNanos { }
        if (visible && !state.loading) playFocus.requestFocus() else rootFocus.requestFocus()
    }
    BackHandler {
        if (visible && !state.paused && !state.loading && state.error == null && !state.ended) {
            showControls = false
        } else model.close(onClosed)
    }
    Box(
        Modifier.fillMaxSize().background(Color.Black)
            .onPreviewKeyEvent { event ->
                if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                interaction++
                when (event.nativeKeyEvent.keyCode) {
                    KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE -> {
                        model.togglePause()
                        showControls = true
                        true
                    }
                    KeyEvent.KEYCODE_MEDIA_PAUSE -> { model.setPaused(true); showControls = true; true }
                    KeyEvent.KEYCODE_MEDIA_PLAY -> { model.setPaused(false); showControls = true; true }
                    KeyEvent.KEYCODE_MEDIA_REWIND -> { model.seek(-30.0); showControls = true; true }
                    KeyEvent.KEYCODE_MEDIA_FAST_FORWARD -> { model.seek(30.0); showControls = true; true }
                    KeyEvent.KEYCODE_DPAD_CENTER, KeyEvent.KEYCODE_DPAD_UP, KeyEvent.KEYCODE_DPAD_DOWN,
                    KeyEvent.KEYCODE_DPAD_LEFT, KeyEvent.KEYCODE_DPAD_RIGHT -> {
                        val wasHidden = !visible
                        showControls = true
                        wasHidden
                    }
                    else -> false
                }
            }.focusRequester(rootFocus).focusable(),
    ) {
        AndroidView(
            factory = { PlayerView(it).apply {
                player = model.player
                useController = false
                isFocusable = false
            } },
            update = { it.player = model.player },
            modifier = Modifier.fillMaxSize(),
        )
        if (visible) Column(
            Modifier.align(Alignment.BottomCenter).fillMaxWidth().background(Color(0xDC111827)).padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(state.item.title)
            Text("${clock(state.position)} / ${clock(state.duration)}")
            state.error?.let { Text(it, color = Color(0xFFFCA5A5)) }
            if (state.loading) CircularProgressIndicator(color = Color(0xFFA5B4FC))
            if (state.ended) Text(stringResource(R.string.ended))
            Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                when {
                    state.error != null -> Button(onClick = model::retry, modifier = Modifier.focusRequester(playFocus)) {
                        Text(stringResource(R.string.retry))
                    }
                    state.ended -> Button(onClick = { model.close(onClosed) }, modifier = Modifier.focusRequester(playFocus)) {
                        Text(stringResource(R.string.back))
                    }
                    else -> Button(
                        onClick = model::togglePause, enabled = !state.loading,
                        modifier = Modifier.focusRequester(playFocus),
                    ) { Text(stringResource(if (state.paused) R.string.play else R.string.pause)) }
                }
                Action(R.string.rewind, !state.loading) { model.seek(-30.0) }
                Action(R.string.forward, !state.loading && !state.ended) { model.seek(30.0) }
                if (state.ended && state.item.next != null) Action(R.string.next_episode) { model.next() }
                Action(R.string.stop) { model.close(onClosed) }
            }
        }
    }
}
