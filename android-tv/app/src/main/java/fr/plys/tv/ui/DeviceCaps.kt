package fr.plys.tv.ui

import android.media.MediaCodecList
import fr.plys.tv.data.ClientCaps

fun deviceCaps(): ClientCaps {
    val codecs = MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos.filter { !it.isEncoder }
    fun supports(mime: String): Boolean = codecs.any { codec ->
        codec.supportedTypes.any { it.equals(mime, ignoreCase = true) }
    }
    val video = mapOf("h264" to "video/avc", "hevc" to "video/hevc", "av1" to "video/av01")
    val audio = mapOf(
        "aac" to "audio/mp4a-latm", "mp3" to "audio/mpeg", "opus" to "audio/opus",
        "flac" to "audio/flac", "ac3" to "audio/ac3", "eac3" to "audio/eac3",
    )
    // MVP targets SDR/1080p. HDR is tone mapped server-side even on HDR-capable TVs.
    return ClientCaps(video.filterValues(::supports).keys.toList(), audio.filterValues(::supports).keys.toList())
}
