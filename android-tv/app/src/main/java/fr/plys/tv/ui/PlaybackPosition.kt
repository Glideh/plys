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

fun clock(seconds: Double): String {
    val value = max(0, seconds.toLong())
    return if (value >= 3600) String.format(Locale.ROOT, "%d:%02d:%02d", value / 3600, value / 60 % 60, value % 60)
    else String.format(Locale.ROOT, "%d:%02d", value / 60, value % 60)
}
