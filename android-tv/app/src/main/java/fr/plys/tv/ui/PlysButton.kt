package fr.plys.tv.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.RowScope
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Border
import androidx.tv.material3.ButtonDefaults

/**
 * Buttons follow the web's `BaseButton`: the primary one is filled with the accent, the others are quiet glass.
 * On a TV the focus must read from the couch, so a focused button fills with a light colour and grows slightly
 * instead of only drawing a thin outline.
 */
@Composable
fun PlysButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    primary: Boolean = false,
    transparent: Boolean = false,
    shape: Shape = Plys.Small,
    contentPadding: PaddingValues = ButtonDefaults.ContentPadding,
    focusedScale: Float = 1.05f,
    pressedScale: Float = 0.98f,
    content: @Composable RowScope.() -> Unit,
) {
    androidx.tv.material3.Button(
        onClick = onClick, modifier = modifier, enabled = enabled,
        shape = ButtonDefaults.shape(shape = shape),
        scale = ButtonDefaults.scale(focusedScale = focusedScale, pressedScale = pressedScale),
        contentPadding = contentPadding,
        border = ButtonDefaults.border(
            border = Border(BorderStroke(1.dp, if (primary || transparent) Color.Transparent else Plys.Hairline), shape = shape),
            focusedBorder = Border(BorderStroke(2.dp, if (primary) Plys.Accent300 else Color.White), shape = shape),
        ),
        colors = if (primary) ButtonDefaults.colors(
            containerColor = Plys.Accent500, contentColor = Plys.Surface950,
            focusedContainerColor = Plys.Accent300, focusedContentColor = Plys.Surface950,
            pressedContainerColor = Plys.Accent400, pressedContentColor = Plys.Surface950,
            disabledContainerColor = Plys.Accent600.copy(alpha = 0.4f), disabledContentColor = Plys.Surface950.copy(alpha = 0.7f),
        ) else ButtonDefaults.colors(
            containerColor = if (transparent) Color.Transparent else Color.White.copy(alpha = 0.07f), contentColor = Plys.Text,
            focusedContainerColor = Plys.Text, focusedContentColor = Plys.Surface950,
            pressedContainerColor = Plys.Muted, pressedContentColor = Plys.Surface950,
            disabledContainerColor = if (transparent) Color.Transparent else Color.White.copy(alpha = 0.03f), disabledContentColor = Plys.Subtle.copy(alpha = 0.75f),
        ),
        content = content,
    )
}
