package fr.plys.tv.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Border
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.MaterialTheme

@Composable
fun PlysButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    content: @Composable RowScope.() -> Unit,
) {
    val palette = MaterialTheme.colorScheme
    val shape = RoundedCornerShape(6.dp)
    androidx.tv.material3.Button(
        onClick = onClick, modifier = modifier, enabled = enabled,
        shape = ButtonDefaults.shape(shape = shape),
        scale = ButtonDefaults.scale(focusedScale = 1f),
        border = ButtonDefaults.border(
            border = Border(BorderStroke(1.dp, palette.onSurface.copy(alpha = 0.1f)), shape = shape),
            focusedBorder = Border(BorderStroke(2.dp, palette.primary), shape = shape),
        ),
        colors = ButtonDefaults.colors(
            containerColor = palette.onSurface.copy(alpha = 0.05f), contentColor = palette.onSurface,
            focusedContainerColor = palette.onSurface.copy(alpha = 0.1f), focusedContentColor = palette.onSurface,
            disabledContainerColor = palette.onSurface.copy(alpha = 0.025f),
            disabledContentColor = palette.onSurfaceVariant.copy(alpha = 0.75f),
        ),
        content = content,
    )
}