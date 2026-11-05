package fr.plys.tv.ui

import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.ExperimentalTextApi
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.shape.RoundedCornerShape
import fr.plys.tv.R
import androidx.tv.material3.LocalContentColor

@OptIn(ExperimentalTextApi::class)
private val nunito = FontFamily(listOf(400, 500, 600, 700, 800).map { weight ->
    Font(R.font.nunito, weight = FontWeight(weight), variationSettings = FontVariation.Settings(FontVariation.weight(weight)))
})

private fun TextStyle.nunito() = copy(fontFamily = nunito, letterSpacing = 0.sp, color = Color.Unspecified)

@Composable
fun PlysTheme(content: @Composable () -> Unit) {
    val background = Color(0xFF10141B)
    val surface = Color(0xFF161B24)
    val foreground = Color(0xFFE2E8F0)
    val secondary = Color(0xFFAAB4C3)
    val accent = Color(0xFF83BFD9)
    val phone = androidx.compose.material3.Typography()
    val tv = androidx.tv.material3.Typography()
    androidx.compose.material3.MaterialTheme(
        typography = androidx.compose.material3.Typography(
            displayLarge = phone.displayLarge.nunito(), displayMedium = phone.displayMedium.nunito(), displaySmall = phone.displaySmall.nunito(),
            headlineLarge = phone.headlineLarge.nunito(), headlineMedium = phone.headlineMedium.nunito(), headlineSmall = phone.headlineSmall.nunito(),
            titleLarge = phone.titleLarge.nunito(), titleMedium = phone.titleMedium.nunito(), titleSmall = phone.titleSmall.nunito(),
            bodyLarge = phone.bodyLarge.nunito(), bodyMedium = phone.bodyMedium.nunito(), bodySmall = phone.bodySmall.nunito(),
            labelLarge = phone.labelLarge.nunito(), labelMedium = phone.labelMedium.nunito(), labelSmall = phone.labelSmall.nunito(),
        ),
        shapes = androidx.compose.material3.Shapes(
            extraSmall = RoundedCornerShape(4.dp), small = RoundedCornerShape(6.dp), medium = RoundedCornerShape(8.dp),
            large = RoundedCornerShape(8.dp), extraLarge = RoundedCornerShape(8.dp),
        ),
        colorScheme = androidx.compose.material3.darkColorScheme(
            background = background, onBackground = foreground,
            surface = surface, onSurface = foreground,
            primary = accent, onPrimary = background,
            surfaceVariant = Color(0xFF1D2430), onSurfaceVariant = secondary, error = Color(0xFFFFB4AB),
        ),
    ) {
        androidx.tv.material3.MaterialTheme(
            typography = androidx.tv.material3.Typography(
                displayLarge = tv.displayLarge.nunito(), displayMedium = tv.displayMedium.nunito(), displaySmall = tv.displaySmall.nunito(),
                headlineLarge = tv.headlineLarge.nunito(), headlineMedium = tv.headlineMedium.nunito(), headlineSmall = tv.headlineSmall.nunito(),
                titleLarge = tv.titleLarge.nunito(), titleMedium = tv.titleMedium.nunito(), titleSmall = tv.titleSmall.nunito(),
                bodyLarge = tv.bodyLarge.nunito(), bodyMedium = tv.bodyMedium.nunito(), bodySmall = tv.bodySmall.nunito(),
                labelLarge = tv.labelLarge.nunito(), labelMedium = tv.labelMedium.nunito(), labelSmall = tv.labelSmall.nunito(),
            ),
            colorScheme = androidx.tv.material3.darkColorScheme(
                background = background, onBackground = foreground,
                surface = surface, onSurface = foreground,
                primary = accent, onPrimary = background,
                surfaceVariant = Color(0xFF1D2430), onSurfaceVariant = secondary, error = Color(0xFFFFB4AB),
            ),
        ) {
            Surface(color = background, contentColor = foreground) {
                CompositionLocalProvider(
                    LocalContentColor provides foreground,
                    androidx.compose.material3.LocalContentColor provides foreground,
                    androidx.tv.material3.LocalTextStyle provides androidx.tv.material3.MaterialTheme.typography.bodyLarge,
                    androidx.compose.material3.LocalTextStyle provides androidx.compose.material3.MaterialTheme.typography.bodyLarge,
                    content = content,
                )
            }
        }
    }
}