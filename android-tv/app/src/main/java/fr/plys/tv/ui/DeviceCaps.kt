package fr.plys.tv.ui

import android.media.MediaCodecList
import android.content.Context
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.exoplayer.audio.AudioSink
import androidx.media3.exoplayer.audio.DefaultAudioSink
import fr.plys.tv.data.ClientCaps

fun playbackAudioAttributes(): AudioAttributes = AudioAttributes.Builder()
    .setUsage(C.USAGE_MEDIA).setContentType(C.AUDIO_CONTENT_TYPE_MOVIE).build()

fun audioClientCaps(video: List<String>, decoded: List<String>, direct: List<String>): ClientCaps {
    val dolby = listOf("ac3", "eac3")
    val preferred = dolby.firstOrNull { it in direct }.orEmpty()
    val decoders = if (preferred.isEmpty()) decoded else decoded.filterNot { it in dolby }
    return ClientCaps(
        videoCodecs = video, audioCodecs = (decoders + direct).distinct(),
        preferredAudioCodec = preferred,
    )
}

@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
fun deviceCaps(context: Context): ClientCaps {
    val codecs = MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos.filter { !it.isEncoder }
    fun supports(mime: String): Boolean = codecs.any { codec ->
        codec.supportedTypes.any { it.equals(mime, ignoreCase = true) }
    }
    val video = mapOf("h264" to "video/avc", "hevc" to "video/hevc", "av1" to "video/av01")
    val audio = mapOf(
        "aac" to "audio/mp4a-latm", "mp3" to "audio/mpeg", "opus" to "audio/opus",
        "flac" to "audio/flac", "ac3" to "audio/ac3", "eac3" to "audio/eac3",
    )
    val sink = DefaultAudioSink.Builder(context).build()
    try {
        sink.setAudioAttributes(playbackAudioAttributes())
        val direct = audio.filterKeys { it == "ac3" || it == "eac3" }.filterValues { mime ->
            sink.getFormatSupport(Format.Builder().setSampleMimeType(mime).setChannelCount(6).setSampleRate(48000).build()) == AudioSink.SINK_FORMAT_SUPPORTED_DIRECTLY
        }.keys.toList()
        return audioClientCaps(video.filterValues(::supports).keys.toList(), audio.filterValues(::supports).keys.toList(), direct)
    } finally { sink.release() }
}
