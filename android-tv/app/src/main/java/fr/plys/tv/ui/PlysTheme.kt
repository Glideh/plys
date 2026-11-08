package fr.plys.tv.ui

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.ExperimentalTextApi
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import androidx.tv.material3.LocalContentColor
import fr.plys.tv.R

/**
 * Design tokens shared with the web client (`web/src/style.css`): blue-grey surfaces, the slate-blue brand accent
 * (oklch, hue 230, chroma 0.1, converted to sRGB) and lifted secondary text. Keep both in sync when the brand changes.
 */
object Plys {
    val Surface950 = Color(0xFF10141B)
    val Surface900 = Color(0xFF161B24)
    val Surface800 = Color(0xFF1D2430)
    val Surface700 = Color(0xFF28303E)
    val Surface600 = Color(0xFF343E4F)

    val Accent300 = Color(0xFF9ADBFC)
    val Accent400 = Color(0xFF75CAF2)
    val Accent500 = Color(0xFF5EB3DA)
    val Accent600 = Color(0xFF3D94BA)

    val Text = Color(0xFFE2E8F0)
    val TextStrong = Color(0xFFF8FAFC)
    val Muted = Color(0xFFAAB4C3)
    val Subtle = Color(0xFF8C97A8)
    val Error = Color(0xFFFFB4AB)
    val Warning = Color(0xFFFCD34D)

    /** Subtle outline of resting cards and fields, like the web's `ring-white/5`. */
    val Hairline = Color.White.copy(alpha = 0.06f)

    val Small = RoundedCornerShape(8.dp)
    val Card = RoundedCornerShape(12.dp)
    val Large = RoundedCornerShape(16.dp)

    /** Gradients of the home library tiles, as on the web home page. */
    val MoviesGradient = Brush.linearGradient(listOf(Accent500.copy(alpha = 0.45f), Color(0xFF0369A1).copy(alpha = 0.3f), Color(0xFF3730A3).copy(alpha = 0.4f)))
    val SeriesGradient = Brush.linearGradient(listOf(Color(0xFF818CF8).copy(alpha = 0.45f), Color(0xFF7C3AED).copy(alpha = 0.3f), Color(0xFF334155).copy(alpha = 0.4f)))
    val OtherGradient = Brush.linearGradient(listOf(Color(0xFFFB7185).copy(alpha = 0.38f), Color(0xFFBE185D).copy(alpha = 0.22f), Color(0xFF334155).copy(alpha = 0.4f)))
}

@OptIn(ExperimentalTextApi::class)
private val nunito = FontFamily(listOf(400, 500, 600, 700, 800).map { weight ->
    Font(R.font.nunito, weight = FontWeight(weight), variationSettings = FontVariation.Settings(FontVariation.weight(weight)))
})

private fun TextStyle.nunito() = copy(fontFamily = nunito, letterSpacing = 0.sp, color = Color.Unspecified)

/** Headings are bold and slightly tightened, like the web's `h1`–`h3`. */
private fun TextStyle.heading(weight: Int = 700) = nunito().copy(fontWeight = FontWeight(weight), letterSpacing = (-0.01).em)

@Composable
fun PlysTheme(content: @Composable () -> Unit) {
    val phone = androidx.compose.material3.Typography()
    val tv = androidx.tv.material3.Typography()
    androidx.compose.material3.MaterialTheme(
        typography = androidx.compose.material3.Typography(
            displayLarge = phone.displayLarge.heading(800), displayMedium = phone.displayMedium.heading(800), displaySmall = phone.displaySmall.heading(),
            headlineLarge = phone.headlineLarge.heading(), headlineMedium = phone.headlineMedium.heading(), headlineSmall = phone.headlineSmall.heading(),
            titleLarge = phone.titleLarge.heading(), titleMedium = phone.titleMedium.heading(600), titleSmall = phone.titleSmall.heading(600),
            bodyLarge = phone.bodyLarge.nunito(), bodyMedium = phone.bodyMedium.nunito(), bodySmall = phone.bodySmall.nunito(),
            labelLarge = phone.labelLarge.nunito(), labelMedium = phone.labelMedium.nunito(), labelSmall = phone.labelSmall.nunito(),
        ),
        shapes = androidx.compose.material3.Shapes(
            extraSmall = RoundedCornerShape(4.dp), small = Plys.Small, medium = Plys.Small,
            large = Plys.Card, extraLarge = Plys.Large,
        ),
        colorScheme = androidx.compose.material3.darkColorScheme(
            background = Plys.Surface950, onBackground = Plys.Text,
            surface = Plys.Surface900, onSurface = Plys.Text,
            primary = Plys.Accent400, onPrimary = Plys.Surface950,
            surfaceVariant = Plys.Surface800, onSurfaceVariant = Plys.Muted, error = Plys.Error,
            outline = Plys.Surface600,
        ),
    ) {
        androidx.tv.material3.MaterialTheme(
            typography = androidx.tv.material3.Typography(
                displayLarge = tv.displayLarge.heading(800), displayMedium = tv.displayMedium.heading(800), displaySmall = tv.displaySmall.heading(),
                headlineLarge = tv.headlineLarge.heading(), headlineMedium = tv.headlineMedium.heading(), headlineSmall = tv.headlineSmall.heading(),
                titleLarge = tv.titleLarge.heading(), titleMedium = tv.titleMedium.heading(600), titleSmall = tv.titleSmall.heading(600),
                bodyLarge = tv.bodyLarge.nunito(), bodyMedium = tv.bodyMedium.nunito(), bodySmall = tv.bodySmall.nunito(),
                labelLarge = tv.labelLarge.nunito(), labelMedium = tv.labelMedium.nunito(), labelSmall = tv.labelSmall.nunito(),
            ),
            colorScheme = androidx.tv.material3.darkColorScheme(
                background = Plys.Surface950, onBackground = Plys.Text,
                surface = Plys.Surface900, onSurface = Plys.Text,
                primary = Plys.Accent400, onPrimary = Plys.Surface950,
                surfaceVariant = Plys.Surface800, onSurfaceVariant = Plys.Muted, error = Plys.Error,
                border = Plys.Surface600,
            ),
        ) {
            Surface(color = Plys.Surface950, contentColor = Plys.Text) {
                CompositionLocalProvider(
                    LocalContentColor provides Plys.Text,
                    androidx.compose.material3.LocalContentColor provides Plys.Text,
                    androidx.tv.material3.LocalTextStyle provides androidx.tv.material3.MaterialTheme.typography.bodyLarge,
                    androidx.compose.material3.LocalTextStyle provides androidx.compose.material3.MaterialTheme.typography.bodyLarge,
                    content = content,
                )
            }
        }
    }
}
