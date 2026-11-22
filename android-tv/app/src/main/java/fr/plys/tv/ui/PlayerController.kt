package fr.plys.tv.ui

import android.app.Application
import android.widget.Toast
import androidx.media3.common.MediaItem
import androidx.media3.common.C
import android.net.Uri
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.upstream.DefaultLoadErrorHandlingPolicy
import fr.plys.tv.R
import fr.plys.tv.data.ClientException
import fr.plys.tv.data.ExternalSubtitle
import fr.plys.tv.data.SubtitleResult
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.PlaybackSession
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.ServerRef
import fr.plys.tv.data.PlaybackSelection
import kotlinx.coroutines.delay
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

data class PlayerState(
    val item: ItemDetail,
    val position: Double,
    val duration: Double,
    val loading: Boolean = true,
    val paused: Boolean = false,
    val ended: Boolean = false,
    val error: String? = null,
    val audioInfo: String? = null,
    val selection: PlaybackSelection = PlaybackSelection(),
)

/** The online subtitle search shown over the player; downloading holds the file being fetched. */
data class SubtitleSearchState(
    val language: String,
    val loading: Boolean = true,
    val results: List<SubtitleResult> = emptyList(),
    val error: String? = null,
    val downloading: Int? = null,
)

@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
class PlayerController(
    private val application: Application,
    private val api: PlysApi,
    item: ItemDetail,
    initialPosition: Double,
    private val source: ServerRef? = null,
    initialSelection: PlaybackSelection = PlaybackSelection(),
) {
    private var selection = initialSelection.normalized(item)
    private val trackLanguages = TrackLanguageStore(application)
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    val player = ExoPlayer.Builder(application).build().apply {
        setAudioAttributes(playbackAudioAttributes(), true)
        setHandleAudioBecomingNoisy(true)
    }
    private val mutableState = MutableStateFlow(PlayerState(
        item, initialPosition, selection.version(item)?.durationSeconds ?: 0.0, selection = selection,
    ))
    val state = mutableState.asStateFlow()
    private val mutableSubtitleSearch = MutableStateFlow<SubtitleSearchState?>(null)
    val subtitleSearch = mutableSubtitleSearch.asStateFlow()
    private var searchJob: kotlinx.coroutines.Job? = null
    private var session: PlaybackSession? = null
    private val operations = Mutex()
    private var foreground = false
    private var closing = false
    private var disposing = false
    private var ready = false
    private var lastReported = -1.0

    init {
        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                when (playbackState) {
                    Player.STATE_READY -> {
                        ready = true
                        val format = player.audioFormat
                        val codec = when (format?.sampleMimeType) {
                            MimeTypes.AUDIO_AAC -> "AAC"
                            MimeTypes.AUDIO_AC3 -> "AC-3"
                            MimeTypes.AUDIO_E_AC3 -> "E-AC-3"
                            else -> format?.sampleMimeType?.removePrefix("audio/")?.uppercase()
                        }
                        val info = if (codec != null && format != null && format.channelCount > 0) application.getString(R.string.audio_stream_info, codec, format.channelCount) else null
                        mutableState.value = mutableState.value.copy(loading = false, audioInfo = info)
                    }
                    Player.STATE_ENDED -> {
                        val duration = if (mutableState.value.duration > 0) mutableState.value.duration
                        else sourcePosition(session?.start ?: 0.0, player.currentPosition, 0.0)
                        mutableState.value = mutableState.value.copy(
                            ended = true, paused = true, position = duration, duration = duration, loading = false,
                        )
                        scope.launch { operations.withLock { attempt { report(ended = true) } } }
                    }
                }
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (!isPlaying && !mutableState.value.loading && !mutableState.value.ended && foreground) {
                    updatePosition()
                    scope.launch { operations.withLock { attempt { report() } } }
                }
            }

            override fun onPlayWhenReadyChanged(playWhenReady: Boolean, reason: Int) {
                if (!mutableState.value.loading && !closing) {
                    mutableState.value = mutableState.value.copy(paused = !playWhenReady)
                }
            }

            override fun onPlayerError(error: PlaybackException) {
                mutableState.value = mutableState.value.copy(
                    loading = false,
                    error = application.getString(R.string.error_playback, error.errorCodeName),
                )
            }
        })
        scope.launch {
            var ticks = 0
            while (true) {
                delay(1000)
                if (session != null && !mutableState.value.loading && !mutableState.value.ended) updatePosition()
                if (++ticks % 10 == 0 && player.isPlaying) {
                    operations.withLock { attempt { report() } }
                }
            }
        }
    }

    fun start() {
        if (foreground || closing || disposing) return
        foreground = true
        if (!mutableState.value.ended) open(mutableState.value.position)
    }

    fun background() {
        if (!foreground || disposing) return
        foreground = false
        if (!mutableState.value.loading && !mutableState.value.ended) updatePosition()
        player.pause()
        scope.launch {
            operations.withLock {
                attempt { report(ended = mutableState.value.ended) }
                player.stop()
                attempt { stopSession() }
            }
        }
    }

    fun togglePause() {
        setPaused(!mutableState.value.paused)
    }

    fun setPaused(paused: Boolean) {
        if (mutableState.value.loading || mutableState.value.ended) return
        mutableState.value = mutableState.value.copy(paused = paused)
        player.playWhenReady = !paused
    }

    /** Seeks relative to the current position, or to the position already requested while a seek is loading. */
    fun seek(delta: Double) {
        if (closing || disposing) return
        if (!mutableState.value.loading) updatePosition()
        open(seekPosition(mutableState.value.position, delta, mutableState.value.duration))
    }

    /**
     * Switches the audio or subtitle track: the server session is reopened at the current position. The picked track
     * also becomes the language preferred for the next titles.
     */
    fun select(choice: PlaybackSelection) {
        if (closing || disposing) return
        val next = choice.normalized(mutableState.value.item)
        if (next == selection) return
        trackLanguages.remember(mutableState.value.item, selection, next)
        selection = next
        mutableState.value = mutableState.value.copy(selection = next)
        if (!mutableState.value.loading && !mutableState.value.ended) updatePosition()
        open(mutableState.value.position)
    }

    fun retry() = open(mutableState.value.position)

    /** Opens the search, or searches again in another language. */
    fun searchSubtitles(language: String) {
        val item = mutableState.value.item
        val version = selection.version(item) ?: return
        mutableSubtitleSearch.value = SubtitleSearchState(language)
        searchJob?.cancel()
        searchJob = scope.launch {
            mutableSubtitleSearch.value = try {
                SubtitleSearchState(language, loading = false, results = api.searchSubtitles(item.id, version.id, language, source))
            } catch (error: ClientException) {
                SubtitleSearchState(language, loading = false, error = application.errorMessage(error))
            }
        }
    }

    fun closeSubtitleSearch() {
        searchJob?.cancel()
        mutableSubtitleSearch.value = null
    }

    /** Downloads a found subtitle and shows it at once. */
    fun downloadSubtitle(result: SubtitleResult) {
        val search = mutableSubtitleSearch.value ?: return
        if (search.downloading != null) return
        val item = mutableState.value.item
        val version = selection.version(item) ?: return
        mutableSubtitleSearch.value = search.copy(downloading = result.fileId, error = null)
        searchJob = scope.launch {
            try {
                val subtitle = api.downloadSubtitle(item.id, version.id, result, source)
                addExternalSubtitle(version.id, subtitle)
                mutableSubtitleSearch.value = null
                select(selection.copy(subtitleIndex = null, externalSubtitleId = subtitle.id))
            } catch (error: ClientException) {
                mutableSubtitleSearch.value = mutableSubtitleSearch.value?.copy(downloading = null, error = application.errorMessage(error))
            }
        }
    }

    private fun addExternalSubtitle(versionId: String, subtitle: ExternalSubtitle) {
        val current = mutableState.value
        val versions = current.item.versions.map { v ->
            if (v.id != versionId || v.externalSubtitles.any { it.id == subtitle.id }) v else v.copy(externalSubtitles = v.externalSubtitles + subtitle)
        }
        mutableState.value = current.copy(item = current.item.copy(versions = versions))
    }

    fun next() {
        if (mutableState.value.loading || closing || disposing) return
        val next = mutableState.value.item.next ?: return
        mutableState.value = mutableState.value.copy(loading = true, error = null)
        scope.launch {
            operations.withLock {
                attempt {
                    attempt(notify = true) { report(ended = mutableState.value.ended) }
                    stopSession()
                    val nextItem = api.item(next.id, source)
                    selection = selection.forNext(mutableState.value.item, nextItem)
                    mutableState.value = PlayerState(
                        nextItem, next.positionSeconds, nextItem.playableVersion?.durationSeconds ?: 0.0, selection = selection,
                    )
                    lastReported = -1.0
                    openSession(next.positionSeconds)
                }
            }
        }
    }

    fun close(onClosed: () -> Unit) {
        if (closing) return
        closing = true
        foreground = false
        if (!mutableState.value.loading && !mutableState.value.ended) updatePosition()
        player.pause()
        scope.launch {
            operations.withLock {
                attempt(notify = true) { report(ended = mutableState.value.ended) }
                player.stop()
                attempt(notify = true) { stopSession() }
            }
            onClosed()
        }
    }

    private fun open(position: Double) {
        if (closing || disposing) return
        if (!mutableState.value.loading && !mutableState.value.ended) updatePosition()
        // The requested position is shown at once and is the base of any further seek while loading.
        mutableState.value = mutableState.value.copy(loading = true, error = null, position = position, ended = false)
        player.pause()
        scope.launch {
            operations.withLock {
                attempt {
                    attempt(notify = true) { report(ended = mutableState.value.ended) }
                    player.stop()
                    stopSession()
                    if (foreground && !closing) openSession(position)
                }
            }
        }
    }

    private suspend fun openSession(position: Double) {
        selection = selection.normalized(mutableState.value.item)
        mutableState.value = mutableState.value.copy(selection = selection)
        val version = selection.version(mutableState.value.item)
            ?: throw ClientException(fr.plys.tv.data.Failure.NOT_FOUND)
        val opened = api.start(version.id, position, deviceCaps(application), source, selection.audioIndex, selection.subtitleIndex, selection.externalSubtitleId)
        player.clearMediaItems()
        session = opened
        ready = false
        if (!foreground || closing) {
            stopSession()
            return
        }
        mutableState.value = mutableState.value.copy(
            position = position, duration = opened.duration, ended = false,
        )
        val item = MediaItem.Builder()
            .setUri(api.mediaUrl(opened.playlistUrl, source))
            .setMimeType(MimeTypes.APPLICATION_M3U8)
        if (opened.subtitleUrl != null && !opened.burnedSubtitle) {
            val url = api.readySubtitleUrl(opened.subtitleUrl, source)
            val language = version.subtitles.firstOrNull { it.index == selection.subtitleIndex }?.language
                ?: version.externalSubtitles.firstOrNull { it.id == selection.externalSubtitleId }?.language
            item.setSubtitleConfigurations(listOf(MediaItem.SubtitleConfiguration.Builder(Uri.parse(url))
                .setMimeType(MimeTypes.TEXT_VTT).setLanguage(language?.takeIf { it.isNotBlank() })
                .setSelectionFlags(C.SELECTION_FLAG_DEFAULT).build()))
        }
        if (!foreground || closing || disposing) { stopSession(); return }
        val mediaSource = DefaultMediaSourceFactory(DefaultHttpDataSource.Factory()
            .setConnectTimeoutMs(15_000).setReadTimeoutMs(30_000))
            .setLoadErrorHandlingPolicy(DefaultLoadErrorHandlingPolicy(6))
            .createMediaSource(item.build())
        // HLS is an EVENT playlist and only contains the portion starting at session.start.
        // Always seek by opening a server session, not by seeking into unproduced HLS segments.
        player.setMediaSource(mediaSource, streamStartMs(position, opened.start))
        player.prepare()
        player.playWhenReady = !mutableState.value.paused
    }

    private fun updatePosition() {
        val current = session ?: return
        mutableState.value = mutableState.value.copy(
            position = sourcePosition(current.start, player.currentPosition, current.duration),
        )
    }

    private suspend fun report(ended: Boolean = false) {
        val current = mutableState.value
        if (session == null || current.duration <= 0 || (!ready && !ended)) return
        val position = if (ended) current.duration else mutableState.value.position
        if (position < 1 || kotlin.math.abs(position - lastReported) < 1) return
        api.progress(current.item.id, position, current.duration, source)
        lastReported = position
    }

    private suspend fun stopSession() {
        val current = session ?: return
        api.stop(current.id, source)
        session = null
    }

    private suspend fun attempt(notify: Boolean = false, action: suspend () -> Unit) {
        try {
            action()
        } catch (error: ClientException) {
            val message = application.errorMessage(error)
            mutableState.value = mutableState.value.copy(loading = false, error = message)
            if (notify) Toast.makeText(application, message, Toast.LENGTH_LONG).show()
        }
    }

    fun dispose() {
        if (disposing) return
        disposing = true
        closing = true
        foreground = false
        if (!mutableState.value.loading && !mutableState.value.ended) updatePosition()
        player.pause()
        // Finish in-flight session creation before releasing it, even when Compose leaves this screen.
        scope.launch {
            try {
                operations.withLock {
                    attempt(notify = true) { report(ended = mutableState.value.ended) }
                    attempt(notify = true) { stopSession() }
                }
            } finally {
                player.release()
                scope.cancel()
            }
        }
    }
}
