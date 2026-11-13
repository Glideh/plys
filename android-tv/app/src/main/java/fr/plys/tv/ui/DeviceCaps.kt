package fr.plys.tv.ui

import android.annotation.SuppressLint
import android.content.Context
import android.hardware.display.DisplayManager
import android.media.MediaCodecInfo
import android.media.MediaCodecInfo.CodecProfileLevel
import android.media.MediaCodecList
import android.os.Build
import android.view.Display
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

/** What the decoders of one codec handle: the largest standard picture height and 10-bit HDR10. */
data class VideoDecoding(val codec: String, val maxHeight: Int, val hdr10: Boolean)

/** Below this, the server transcodes to its own 1080p cap anyway. */
private const val MIN_HEIGHT = 1080

/**
 * Lets the server copy what the device plays natively instead of transcoding it to 1080p SDR. The height
 * limit is the one every announced codec reaches, since the server applies it to all of them. HDR is
 * announced only when the connected display shows HDR10 and the HDR codecs decode it; otherwise the server
 * tone maps.
 */
fun videoClientCaps(caps: ClientCaps, decoders: List<VideoDecoding>, displayHdr10: Boolean): ClientCaps {
    val announced = decoders.filter { it.codec in caps.videoCodecs }
    val maxHeight = announced.minOfOrNull { it.maxHeight }?.coerceAtLeast(MIN_HEIGHT) ?: MIN_HEIGHT
    // H.264 is never copied in 10 bits, so only HEVC and AV1 carry HDR.
    val hdrCodecs = announced.filter { it.codec != "h264" }
    val hdr = displayHdr10 && hdrCodecs.isNotEmpty() && hdrCodecs.all { it.hdr10 }
    return caps.copy(maxHeight = maxHeight, hdr = hdr)
}

private val videoMimes = mapOf("h264" to "video/avc", "hevc" to "video/hevc", "av1" to "video/av01")

// Inlined constants: older decoders simply never report the newer profiles.
@SuppressLint("InlinedApi")
private val hdr10Profiles = mapOf(
    "hevc" to setOf(CodecProfileLevel.HEVCProfileMain10HDR10, CodecProfileLevel.HEVCProfileMain10HDR10Plus),
    "av1" to setOf(CodecProfileLevel.AV1ProfileMain10HDR10, CodecProfileLevel.AV1ProfileMain10HDR10Plus),
)

private val standardSizes = listOf(3840 to 2160, 2560 to 1440, 1920 to 1080, 1280 to 720)

/** Software decoders may accept 4K but cannot decode it in real time on a TV device. */
private fun MediaCodecInfo.isHardware(): Boolean =
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) isHardwareAccelerated
    else !name.startsWith("OMX.google.") && !name.startsWith("c2.android.")

private fun videoDecoding(codecs: List<MediaCodecInfo>, codec: String, mime: String): VideoDecoding? {
    var maxHeight = 0
    var hdr10 = false
    for (info in codecs) {
        val type = info.supportedTypes.firstOrNull { it.equals(mime, ignoreCase = true) } ?: continue
        val capabilities = runCatching { info.getCapabilitiesForType(type) }.getOrNull() ?: continue
        val video = capabilities.videoCapabilities ?: continue
        val height = standardSizes.firstOrNull { (w, h) ->
            (h <= MIN_HEIGHT || info.isHardware()) && runCatching { video.isSizeSupported(w, h) }.getOrDefault(false)
        }?.second ?: 0
        maxHeight = maxOf(maxHeight, height)
        if (info.isHardware() && capabilities.profileLevels.any { it.profile in hdr10Profiles[codec].orEmpty() }) hdr10 = true
    }
    return if (maxHeight > 0) VideoDecoding(codec, maxHeight, hdr10) else null
}

/** On a TV box, these are the capabilities of the TV it is plugged into, as reported over HDMI. */
@Suppress("DEPRECATION")
private fun displaySupportsHdr10(context: Context): Boolean {
    val display = context.getSystemService(DisplayManager::class.java)?.getDisplay(Display.DEFAULT_DISPLAY) ?: return false
    val types = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) display.mode.supportedHdrTypes
    else display.hdrCapabilities?.supportedHdrTypes
    return types?.contains(Display.HdrCapabilities.HDR_TYPE_HDR10) == true
}

@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
fun deviceCaps(context: Context): ClientCaps {
    val codecs = MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos.filter { !it.isEncoder }
    fun supports(mime: String): Boolean = codecs.any { codec ->
        codec.supportedTypes.any { it.equals(mime, ignoreCase = true) }
    }
    val audio = mapOf(
        "aac" to "audio/mp4a-latm", "mp3" to "audio/mpeg", "opus" to "audio/opus",
        "flac" to "audio/flac", "ac3" to "audio/ac3", "eac3" to "audio/eac3",
    )
    val sink = DefaultAudioSink.Builder(context).build()
    val caps = try {
        sink.setAudioAttributes(playbackAudioAttributes())
        val direct = audio.filterKeys { it == "ac3" || it == "eac3" }.filterValues { mime ->
            sink.getFormatSupport(Format.Builder().setSampleMimeType(mime).setChannelCount(6).setSampleRate(48000).build()) == AudioSink.SINK_FORMAT_SUPPORTED_DIRECTLY
        }.keys.toList()
        audioClientCaps(videoMimes.filterValues(::supports).keys.toList(), audio.filterValues(::supports).keys.toList(), direct)
    } finally { sink.release() }
    val decoders = videoMimes.mapNotNull { (codec, mime) -> videoDecoding(codecs, codec, mime) }
    return videoClientCaps(caps, decoders, displaySupportsHdr10(context))
}
