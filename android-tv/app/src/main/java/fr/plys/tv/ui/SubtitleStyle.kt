package fr.plys.tv.ui

import android.content.Context
import android.graphics.Color
import android.graphics.Typeface
import android.os.Build
import androidx.core.content.res.ResourcesCompat
import androidx.media3.ui.CaptionStyleCompat
import fr.plys.tv.R

/** Subtitle text sizes; the keys are the ones the web player stores. */
enum class SubtitleSize(val key: String, val sp: Float, val label: Int) {
    SMALL("small", 24f, R.string.subtitle_size_small),
    MEDIUM("medium", 30f, R.string.subtitle_size_medium),
    LARGE("large", 38f, R.string.subtitle_size_large),
    EXTRA_LARGE("xlarge", 46f, R.string.subtitle_size_extra_large),
}

/** What keeps subtitles readable over the picture: a black outline around the letters, or a dark box behind them. */
enum class SubtitleBackground(val key: String, val label: Int) {
    OUTLINE("outline", R.string.subtitle_background_outline),
    BOX("box", R.string.subtitle_background_box),
}

data class SubtitleStyle(val size: SubtitleSize = SubtitleSize.MEDIUM, val background: SubtitleBackground = SubtitleBackground.OUTLINE)

/** Reads stored keys; an unknown or missing one falls back to the default. */
internal fun subtitleStyleOf(size: String?, background: String?) = SubtitleStyle(
    SubtitleSize.entries.firstOrNull { it.key == size } ?: SubtitleSize.MEDIUM,
    SubtitleBackground.entries.firstOrNull { it.key == background } ?: SubtitleBackground.OUTLINE,
)

/** Dark box behind the text, at 80% opacity. */
internal const val SUBTITLE_BOX_COLOR = 0xCC000000.toInt()

@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
internal fun captionStyle(background: SubtitleBackground, typeface: Typeface): CaptionStyleCompat = when (background) {
    SubtitleBackground.OUTLINE -> CaptionStyleCompat(Color.WHITE, Color.TRANSPARENT, Color.TRANSPARENT,
        CaptionStyleCompat.EDGE_TYPE_OUTLINE, Color.BLACK, typeface)
    SubtitleBackground.BOX -> CaptionStyleCompat(Color.WHITE, SUBTITLE_BOX_COLOR, Color.TRANSPARENT,
        CaptionStyleCompat.EDGE_TYPE_NONE, Color.TRANSPARENT, typeface)
}

/** The subtitle style chosen on this device, kept in its local preferences. */
class SubtitleStyleStore(context: Context) {
    private val preferences = context.getSharedPreferences("player", Context.MODE_PRIVATE)

    fun read(): SubtitleStyle = subtitleStyleOf(preferences.getString(SIZE, null), preferences.getString(BACKGROUND, null))

    fun save(style: SubtitleStyle) {
        preferences.edit().putString(SIZE, style.size.key).putString(BACKGROUND, style.background.key).apply()
    }

    private companion object {
        const val SIZE = "subtitle_size"
        const val BACKGROUND = "subtitle_background"
    }
}

/** Nunito's extra bold, thicker than the interface text so that subtitles stay legible over any picture. */
internal fun subtitleTypeface(context: Context): Typeface {
    val nunito = ResourcesCompat.getFont(context, R.font.nunito) ?: Typeface.DEFAULT
    // Nunito is a variable font: pick its weight rather than a synthetic bold where the platform allows it.
    return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) Typeface.create(nunito, 800, false) else Typeface.create(nunito, Typeface.BOLD)
}

/** Space kept under subtitles while the controls are hidden, as a fraction of the screen height. */
internal const val SUBTITLE_BOTTOM_MARGIN = 0.06f

/**
 * Bottom padding of the subtitle view. The view sits in the picture's frame, so a letterboxed picture already leaves
 * [gapBelow] pixels between it and the bottom of the screen. While the controls show, subtitles move up just above
 * them, whatever the picture's shape, instead of the controls' whole height above the picture.
 */
internal fun subtitleBottomPadding(screenHeight: Int, gapBelow: Int, controlsHeight: Int?): Int {
    val margin = (screenHeight * SUBTITLE_BOTTOM_MARGIN).toInt()
    return if (controlsHeight == null) margin else maxOf(margin, controlsHeight - gapBelow.coerceAtLeast(0))
}
