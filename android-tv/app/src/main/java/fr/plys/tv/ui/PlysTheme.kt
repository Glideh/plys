package fr.plys.tv.ui

import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.Color
import androidx.tv.material3.LocalContentColor

@Composable
fun PlysTheme(content: @Composable () -> Unit) {
    val background = Color(0xFF111315)
    val surface = Color(0xFF23272B)
    val foreground = Color(0xFFF2F4F7)
    val accent = Color(0xFFA5B4FC)
    androidx.compose.material3.MaterialTheme(
        colorScheme = androidx.compose.material3.darkColorScheme(
            background = background, onBackground = foreground,
            surface = surface, onSurface = foreground,
            primary = accent, onPrimary = background,
            onSurfaceVariant = Color(0xFFCBD0D8), error = Color(0xFFFFB4AB),
        ),
    ) {
        androidx.tv.material3.MaterialTheme(
            colorScheme = androidx.tv.material3.darkColorScheme(
                background = background, onBackground = foreground,
                surface = surface, onSurface = foreground,
                primary = accent, onPrimary = background,
                onSurfaceVariant = Color(0xFFCBD0D8), error = Color(0xFFFFB4AB),
            ),
        ) {
            Surface(color = background, contentColor = foreground) {
                CompositionLocalProvider(LocalContentColor provides foreground, content = content)
            }
        }
    }
}