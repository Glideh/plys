package fr.plys.tv.ui

import java.util.Locale
import kotlin.math.max

fun sourcePosition(start: Double, streamPositionMs: Long, duration: Double): Double {
    val position = max(0.0, start + max(0L, streamPositionMs) / 1000.0)
    return if (duration > 0) position.coerceAtMost(duration) else position
}

fun seekPosition(position: Double, delta: Double, duration: Double): Double {
    val target = max(0.0, position + delta)
    return if (duration > 0) target.coerceAtMost(max(0.0, duration - 1)) else target
}

/**
 * Start position in the HLS stream of a session that starts at [start]. The playlist is an EVENT playlist that grows
 * while ffmpeg works, so Media3 treats it as live and replaces a start position of exactly 0 (the default position of
 * its placeholder timeline) with the live window default, a few segments in. One millisecond keeps the request.
 */
fun streamStartMs(position: Double, start: Double): Long =
    max(MIN_STREAM_START_MS, ((position - start).coerceAtLeast(0.0) * 1000).toLong())

private const val MIN_STREAM_START_MS = 1L

fun clock(seconds: Double): String {
    val value = max(0, seconds.toLong())
    return if (value >= 3600) String.format(Locale.ROOT, "%d:%02d:%02d", value / 3600, value / 60 % 60, value % 60)
    else String.format(Locale.ROOT, "%d:%02d", value / 60, value % 60)
}
