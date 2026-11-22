package fr.plys.tv.ui

import android.text.format.DateFormat
import android.util.TypedValue
import android.view.KeyEvent
import android.view.WindowManager
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Audiotrack
import androidx.compose.material.icons.outlined.Forward10
import androidx.compose.material.icons.outlined.Pause
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.Replay10
import androidx.compose.material.icons.outlined.SkipNext
import androidx.compose.material.icons.outlined.Subtitles
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableDoubleStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.media3.ui.PlayerView
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import fr.plys.tv.MainActivity
import fr.plys.tv.PlysApplication
import fr.plys.tv.R
import fr.plys.tv.data.ClientException
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.PlaybackSelection
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.ServerRef
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.util.Date
import fr.plys.tv.ui.PlysButton as Button

/** Seconds moved by one left or right press while the controls are hidden, or by the rewind and forward buttons. */
private const val DPAD_SEEK_SECONDS = 10.0

/** Seconds moved by one press of the remote's seek keys (rewind, fast forward, previous, next). */
private const val SKIP_SEEK_SECONDS = 60.0

/** Seconds before the next episode starts by itself at the end of one. */
private const val AUTO_NEXT_SECONDS = 10

/** Delay after the last seek press before the accumulated seek is applied. */
private const val DPAD_SEEK_DELAY_MS = 700L
private const val SEEK_PREVIEW_FADE_MS = 250
private data class SeekPreview(val offset: Double, val target: Double)

/** What the subtitle view needs from the composition, read again each time the picture's frame is laid out. */
@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
private class SubtitlePlacement {
    /** Height of the controls while they show, null while they are hidden. */
    var controlsHeight: Int? = null
    var style: SubtitleStyle? = null

    fun place(playerView: PlayerView) {
        val view = playerView.subtitleView ?: return
        val screen = IntArray(2).also(playerView::getLocationInWindow)
        val frame = IntArray(2).also(view::getLocationInWindow)
        val gap = (screen[1] + playerView.height) - (frame[1] + view.height)
        val bottom = subtitleBottomPadding(playerView.height, gap, controlsHeight)
        if (view.paddingBottom != bottom) view.setPadding(0, 0, 0, bottom)
    }
}
internal fun playbackControlsVisible(requested: Boolean, loading: Boolean, seekInProgress: Boolean): Boolean =
    requested || (loading && !seekInProgress)

@Composable
@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
fun PlaybackScreen(item: ItemDetail, initialPosition: Double, api: PlysApi, onClosed: () -> Unit, source: ServerRef? = null, selection: PlaybackSelection = PlaybackSelection()) {
    val context = LocalContext.current
    val activity = context as MainActivity
    val model = remember(source?.serverId, item.id, selection) {
        PlayerController(context.applicationContext as PlysApplication, api, item, initialPosition, source, selection)
    }
    DisposableEffect(model) { onDispose { model.dispose() } }
    val state by model.state.collectAsStateWithLifecycle()
    val subtitleSearch by model.subtitleSearch.collectAsStateWithLifecycle()
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
    var controlsHeight by remember { mutableIntStateOf(0) }
    var interaction by remember { mutableIntStateOf(0) }
    var menu by remember { mutableStateOf("") }
    val subtitleStore = remember { SubtitleStyleStore(context.applicationContext) }
    var subtitleStyle by remember { mutableStateOf(subtitleStore.read()) }
    fun setSubtitleStyle(style: SubtitleStyle) {
        subtitleStyle = style
        subtitleStore.save(style)
        menu = ""
    }
    val placement = remember { SubtitlePlacement() }
    val typeface = remember { subtitleTypeface(context) }
    val scope = rememberCoroutineScope()
    // The preference is the user's, shared with the web player; cancelling the countdown only skips this episode.
    var autoplay by remember { mutableStateOf(api.autoplayNext) }
    var cancelled by remember(state.item.id) { mutableStateOf(false) }
    var countdown by remember(state.item.id) { mutableIntStateOf(AUTO_NEXT_SECONDS) }
    val upNext = state.ended && state.item.next != null && autoplay && !cancelled && state.error == null && !state.loading
    fun toggleAutoplay() {
        val wanted = !autoplay
        autoplay = wanted
        scope.launch { autoplay = try { api.setAutoplayNext(wanted) } catch (_: ClientException) { !wanted } }
    }
    LaunchedEffect(upNext) {
        if (!upNext) return@LaunchedEffect
        countdown = AUTO_NEXT_SECONDS
        while (countdown > 0) { delay(1000); countdown-- }
        model.next()
    }
    // Every seek press (left and right while the controls are hidden, the remote's seek keys, the rewind and forward
    // buttons) adds to one offset, applied once the presses stop: one server session is opened, not one per press.
    var pendingSeek by remember { mutableDoubleStateOf(0.0) }
    var seekPresses by remember { mutableIntStateOf(0) }
    var seekPreview by remember { mutableStateOf<SeekPreview?>(null) }
    var seekInProgress by remember { mutableStateOf(false) }
    val controlsRequested = showControls || menu.isNotEmpty() || subtitleSearch != null || state.paused || state.error != null || state.ended
    fun queueSeek(delta: Double) {
        pendingSeek += delta
        seekPreview = SeekPreview(
            offset = pendingSeek,
            target = (state.position + pendingSeek).coerceIn(0.0, state.duration.takeIf { it > 0 } ?: Double.MAX_VALUE),
        )
        seekPresses++
    }
    val rootFocus = remember { FocusRequester() }
    val playFocus = remember { FocusRequester() }
    val visible = playbackControlsVisible(controlsRequested, state.loading, seekInProgress)
    LaunchedEffect(interaction, state.paused, state.loading, state.error, state.ended, menu, subtitleSearch != null) {
        if (!state.paused && !state.loading && state.error == null && !state.ended && menu.isEmpty() && subtitleSearch == null) {
            delay(5000)
            showControls = false
        }
    }
    LaunchedEffect(seekPresses) {
        if (seekPresses == 0) return@LaunchedEffect
        delay(DPAD_SEEK_DELAY_MS)
        seekInProgress = true
        model.seek(pendingSeek)
        pendingSeek = 0.0
        seekPresses = 0
    }
    LaunchedEffect(seekPresses) {
        if (seekPresses == 0 && seekPreview != null) {
            delay(SEEK_PREVIEW_FADE_MS.toLong())
            seekPreview = null
        }
    }
    LaunchedEffect(state.loading, seekPresses) {
        if (seekInProgress && !state.loading && seekPresses == 0) seekInProgress = false
    }
    // Focus goes to the first button when the controls appear, or when an error or the end replaces it; a seek
    // keeps the focus on the button that was pressed, so it can be pressed again.
    LaunchedEffect(visible, state.ended, state.error != null, upNext) {
        androidx.compose.runtime.withFrameNanos { }
        if (visible) playFocus.requestFocus() else rootFocus.requestFocus()
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
                    KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE -> { model.togglePause(); showControls = true; true }
                    KeyEvent.KEYCODE_MEDIA_PAUSE -> { model.setPaused(true); showControls = true; true }
                    KeyEvent.KEYCODE_MEDIA_PLAY -> { model.setPaused(false); showControls = true; true }
                    // The remote's seek keys move by a minute, whether the controls are shown or not: they move no focus.
                    KeyEvent.KEYCODE_MEDIA_REWIND, KeyEvent.KEYCODE_MEDIA_PREVIOUS -> { queueSeek(-SKIP_SEEK_SECONDS); true }
                    KeyEvent.KEYCODE_MEDIA_FAST_FORWARD, KeyEvent.KEYCODE_MEDIA_NEXT -> { queueSeek(SKIP_SEEK_SECONDS); true }
                    // Hidden controls: left and right move through the video instead of opening the controls.
                    KeyEvent.KEYCODE_DPAD_LEFT, KeyEvent.KEYCODE_DPAD_RIGHT -> {
                        if (visible) false else {
                            queueSeek(if (event.nativeKeyEvent.keyCode == KeyEvent.KEYCODE_DPAD_LEFT) -DPAD_SEEK_SECONDS else DPAD_SEEK_SECONDS)
                            true
                        }
                    }
                    KeyEvent.KEYCODE_DPAD_CENTER, KeyEvent.KEYCODE_ENTER, KeyEvent.KEYCODE_DPAD_UP, KeyEvent.KEYCODE_DPAD_DOWN -> {
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
                val playerView = this
                subtitleView?.apply {
                    // Cues and stream styles must not change the chosen size, nor the controls' padding.
                    setApplyEmbeddedFontSizes(false)
                    // The frame follows the picture's shape once the video size is known: the padding is computed again.
                    addOnLayoutChangeListener { view, _, _, _, _, _, _, _, _ -> view.post { placement.place(playerView) } }
                }
            } },
            update = {
                it.player = model.player
                if (placement.style != subtitleStyle) {
                    placement.style = subtitleStyle
                    it.subtitleView?.setFixedTextSize(TypedValue.COMPLEX_UNIT_SP, subtitleStyle.size.sp)
                    it.subtitleView?.setStyle(captionStyle(subtitleStyle.background, typeface))
                }
                // WebVTT cues sit on the last line, which ignores the bottom padding fraction: a padding keeps them
                // above the controls, or off the bottom edge that TVs may crop.
                placement.controlsHeight = if (visible) controlsHeight else null
                placement.place(it)
            },
            modifier = Modifier.fillMaxSize(),
        )
        AnimatedVisibility(
            visible = !visible && seekPreview != null,
            enter = fadeIn(animationSpec = tween(SEEK_PREVIEW_FADE_MS)),
            exit = fadeOut(animationSpec = tween(SEEK_PREVIEW_FADE_MS)),
            modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().padding(start = 48.dp, end = 48.dp, bottom = 28.dp),
        ) {
            SeekProgress(
                preview = seekPreview ?: return@AnimatedVisibility,
                duration = state.duration,
            )
        }
        // Scrims keep the text readable over bright pictures without hiding the video behind a solid bar.
        if (visible) {
            Box(Modifier.align(Alignment.TopCenter).fillMaxWidth().height(160.dp)
                .background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.75f), Color.Transparent))))
            Box(Modifier.align(Alignment.BottomCenter).fillMaxWidth().fillMaxHeight(0.45f)
                .background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.9f)))))
            Column(Modifier.align(Alignment.TopStart).padding(horizontal = 48.dp, vertical = 32.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                val context = listOfNotNull(
                    state.item.ancestors.firstOrNull()?.title?.takeIf { it.isNotBlank() },
                    episodeCode(state.item.seasonNumber.takeIf { state.item.kind == "episode" }, state.item.episodeNumber),
                ).joinToString(" · ")
                if (context.isNotBlank()) Text(context, style = MaterialTheme.typography.titleMedium, color = Plys.Accent300)
                val year = playerYear(state.item)
                Text(buildAnnotatedString {
                    append(detailTitle(state.item))
                    if (year != null) withStyle(SpanStyle(color = Plys.Muted)) { append("  $year") }
                }, style = MaterialTheme.typography.headlineMedium, color = Color.White)
                // Under the title rather than over the middle of the picture, which stays visible while paused.
                pauseLogo(state)?.let { logo ->
                    AsyncImage(model = api.posterUrl(logo, source), contentDescription = detailTitle(state.item),
                        modifier = Modifier.padding(top = 16.dp).fillMaxWidth(0.3f).height(110.dp),
                        alignment = Alignment.TopStart, contentScale = ContentScale.Fit)
                }
            }
        }
        if (visible) Column(
            Modifier.align(Alignment.BottomCenter).fillMaxWidth().onSizeChanged { controlsHeight = it.height }
                .padding(horizontal = 48.dp, vertical = 28.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            val version = state.selection.version(state.item)
            val streamInfo = listOfNotNull(versionSummary(state.item, state.selection).takeIf { it.isNotBlank() }, state.audioInfo).joinToString(" · ")
            Progress(
                position = seekPreview?.target ?: state.position,
                duration = state.duration,
                showEnd = !state.ended && seekPreview == null,
                showCursor = seekPreview != null,
                streamInfo = streamInfo,
                seekOffset = seekPreview?.let { seekOffset(it.offset) },
            )
            state.error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            if (state.ended && !upNext) Text(stringResource(R.string.ended), color = Plys.Muted)
            if (upNext) UpNext(countdown, Modifier.focusRequester(playFocus), playNow = model::next, cancel = { cancelled = true })
            else Row(horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
                when {
                    state.error != null -> IconAction(Icons.Outlined.Refresh, R.string.retry, Modifier.focusRequester(playFocus), onClick = model::retry)
                    state.ended -> IconAction(Icons.AutoMirrored.Outlined.ArrowBack, R.string.back, Modifier.focusRequester(playFocus)) { model.close(onClosed) }
                    // Stays enabled while loading (pausing is ignored then) so it never loses the focus.
                    else -> IconAction(if (state.paused) Icons.Outlined.PlayArrow else Icons.Outlined.Pause, if (state.paused) R.string.play else R.string.pause,
                        Modifier.focusRequester(playFocus), primary = true, onClick = model::togglePause)
                }
                // Presses add up like left and right; seeking while a seek loads moves from the position already requested.
                IconAction(Icons.Outlined.Replay10, R.string.rewind) { queueSeek(-DPAD_SEEK_SECONDS) }
                IconAction(Icons.Outlined.Forward10, R.string.forward, enabled = !state.ended) { queueSeek(DPAD_SEEK_SECONDS) }
                if (version != null && version.audio.size > 1) IconAction(Icons.Outlined.Audiotrack, R.string.audio) { menu = "audio" }
                if (version != null && (version.subtitles.isNotEmpty() || version.externalSubtitles.isNotEmpty() || state.item.subtitleSearch)) IconAction(Icons.Outlined.Subtitles, R.string.subtitles) { menu = "subtitles" }
                if (state.item.next != null) IconAction(Icons.Outlined.SkipNext, R.string.next_episode) { model.next() }
                if (state.item.next != null) AutoplayToggle(autoplay, ::toggleAutoplay)
                if (state.loading) CircularProgressIndicator(Modifier.size(28.dp), color = Plys.Accent400, strokeWidth = 3.dp)
            }
        }
    }
    val version = state.selection.version(state.item)
    if (version != null) when (menu) {
        "audio" -> ChoiceDialog(stringResource(R.string.audio), { menu = "" }) {
            items(version.audio, key = { it.index }) { track ->
                Choice(track.trackLabel(), track.index == state.selection.audioIndex) { model.select(state.selection.copy(audioIndex = track.index)); menu = "" }
            }
        }
        "subtitles" -> ChoiceDialog(stringResource(R.string.subtitles), { menu = "" }) {
            val off = state.selection.subtitleIndex == null && state.selection.externalSubtitleId == null
            item { Choice(stringResource(R.string.no_subtitles), off) { model.select(state.selection.copy(subtitleIndex = null, externalSubtitleId = null)); menu = "" } }
            items(version.subtitles, key = { it.index }) { track ->
                Choice(track.trackLabel(), track.index == state.selection.subtitleIndex) {
                    model.select(state.selection.copy(subtitleIndex = track.index, externalSubtitleId = null)); menu = ""
                }
            }
            items(version.externalSubtitles, key = { it.id }) { subtitle ->
                Choice(subtitle.label(), subtitle.id == state.selection.externalSubtitleId) {
                    model.select(state.selection.copy(subtitleIndex = null, externalSubtitleId = subtitle.id)); menu = ""
                }
            }
            if (state.item.subtitleSearch) item {
                Choice(stringResource(R.string.subtitles_search), false) { menu = ""; model.searchSubtitles(defaultSubtitleLanguage()) }
            }
            item { Choice(stringResource(R.string.subtitle_size_value, stringResource(subtitleStyle.size.label)), false) { menu = "subtitle_size" } }
            item { Choice(stringResource(R.string.subtitle_background_value, stringResource(subtitleStyle.background.label)), false) { menu = "subtitle_background" } }
        }
        "subtitle_size" -> ChoiceDialog(stringResource(R.string.subtitle_size), { menu = "subtitles" }) {
            items(SubtitleSize.entries) { size ->
                Choice(stringResource(size.label), size == subtitleStyle.size) { setSubtitleStyle(subtitleStyle.copy(size = size)) }
            }
        }
        "subtitle_background" -> ChoiceDialog(stringResource(R.string.subtitle_background), { menu = "subtitles" }) {
            items(SubtitleBackground.entries) { background ->
                Choice(stringResource(background.label), background == subtitleStyle.background) { setSubtitleStyle(subtitleStyle.copy(background = background)) }
            }
        }
    }
    subtitleSearch?.let { SubtitleSearchDialog(it, model) }
}

/** Seek text and the player's normal timeline, kept at the same position with or without controls. */
@Composable
private fun SeekProgress(preview: SeekPreview, duration: Double) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(seekOffset(preview.offset), modifier = Modifier.align(Alignment.End),
            style = MaterialTheme.typography.titleMedium, color = Color.White)
        PlaybackTimeline(preview.target, duration, showCursor = true)
        Row(Modifier.fillMaxWidth()) {
            Text("${clock(preview.target)} / ${clock(duration)}", style = MaterialTheme.typography.bodyMedium,
                color = Color.White, modifier = Modifier.weight(1f))
        }
    }
}

@Composable
private fun seekOffset(offset: Double): String {
    val sign = if (offset < 0) "−" else "+"
    return stringResource(R.string.seek_by, sign + kotlin.math.abs(offset).toInt())
}

/** Films and videos show their year next to the title; episodes and parts already carry their context. */
internal fun playerYear(item: ItemDetail): Int? = item.year?.takeIf { item.kind != "episode" && it > 0 }

/** The title logo shown over the paused video, or null while it plays, loads, has ended or failed. */
internal fun pauseLogo(state: PlayerState): String? =
    state.item.logoUrl?.takeIf { it.isNotBlank() && state.paused && !state.loading && !state.ended && state.error == null }

@Composable
private fun UpNext(countdown: Int, modifier: Modifier, playNow: () -> Unit, cancel: () -> Unit) {
    Row(Modifier.background(Plys.Surface900.copy(alpha = 0.92f), Plys.Card).padding(horizontal = 20.dp, vertical = 14.dp),
        horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.width(260.dp)) {
            Text(stringResource(R.string.up_next), style = MaterialTheme.typography.titleMedium, color = Plys.TextStrong)
            Text(stringResource(R.string.up_next_in, countdown), style = MaterialTheme.typography.bodySmall, color = Plys.Muted)
            LinearProgressIndicator(progress = { 1f - countdown / AUTO_NEXT_SECONDS.toFloat() }, modifier = Modifier.padding(top = 8.dp).fillMaxWidth().height(4.dp),
                color = Plys.Accent400, trackColor = Color.White.copy(alpha = 0.15f))
        }
        Action(R.string.play_next_now, modifier = modifier, primary = true, onClick = playNow)
        Action(R.string.cancel, onClick = cancel)
    }
}

/** Switches the auto next preference, filled with the accent while it is on, like the web player's toggle. */
@Composable
private fun AutoplayToggle(enabled: Boolean, onToggle: () -> Unit) {
    val state = stringResource(if (enabled) R.string.autoplay_on else R.string.autoplay_off)
    Button(onClick = onToggle, primary = enabled, shape = androidx.compose.foundation.shape.CircleShape,
        modifier = Modifier.semantics { role = Role.Switch; toggleableState = ToggleableState(enabled); stateDescription = state }) {
        Text(stringResource(R.string.autoplay), style = MaterialTheme.typography.labelLarge)
    }
}

/** Position, duration and, while playing, the time left and the time the video will end. */
@Composable
private fun Progress(
    position: Double,
    duration: Double,
    showEnd: Boolean,
    showCursor: Boolean = false,
    streamInfo: String = "",
    seekOffset: String? = null,
) {
    val context = LocalContext.current
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(
            Modifier.fillMaxWidth().height(28.dp),
            horizontalArrangement = Arrangement.spacedBy(16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(streamInfo, modifier = Modifier.weight(1f), style = MaterialTheme.typography.bodySmall,
                color = Plys.Muted, maxLines = 1, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis)
            if (seekOffset != null) Text(seekOffset, style = MaterialTheme.typography.titleMedium, color = Color.White)
        }
        PlaybackTimeline(position, duration, showCursor)
        val remaining = (duration - position).coerceAtLeast(0.0)
        Row(Modifier.fillMaxWidth()) {
            Text("${clock(position)} / ${clock(duration)}", style = MaterialTheme.typography.bodyMedium, color = Color.White, modifier = Modifier.weight(1f))
            if (showEnd && duration > 0) Text("−${clock(remaining)} · " + stringResource(R.string.ends_at,
                DateFormat.getTimeFormat(context).format(Date(System.currentTimeMillis() + (remaining * 1000).toLong()))),
                style = MaterialTheme.typography.bodyMedium, color = Plys.Muted)
        }
    }
}

@Composable
private fun PlaybackTimeline(position: Double, duration: Double, showCursor: Boolean = false) {
    if (duration <= 0) return
    val progress = (position / duration).toFloat().coerceIn(0f, 1f)
    BoxWithConstraints(
        Modifier.fillMaxWidth().height(10.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        Box(Modifier.fillMaxWidth().height(6.dp).background(Color.White.copy(alpha = 0.2f), RoundedCornerShape(3.dp)))
        Box(Modifier.fillMaxWidth(progress).height(6.dp).background(Plys.Accent400, RoundedCornerShape(3.dp)))
        if (showCursor) Box(
            Modifier.offset(x = (maxWidth - 10.dp) * progress).size(10.dp)
                .background(Color.White, androidx.compose.foundation.shape.CircleShape),
        )
    }
}

@Composable
private fun IconAction(icon: ImageVector, label: Int, modifier: Modifier = Modifier, enabled: Boolean = true, primary: Boolean = false, onClick: () -> Unit) {
    Button(onClick = onClick, modifier = modifier, enabled = enabled, primary = primary, shape = androidx.compose.foundation.shape.CircleShape,
        contentPadding = androidx.compose.foundation.layout.PaddingValues(14.dp)) {
        Icon(icon, contentDescription = stringResource(label), modifier = Modifier.size(28.dp), tint = androidx.tv.material3.LocalContentColor.current)
    }
}
