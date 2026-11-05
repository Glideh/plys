package fr.plys.tv.ui

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.ServerRef

@Composable
fun PosterTile(
    title: String, image: String?, api: PlysApi, modifier: Modifier = Modifier,
    subtitle: String? = null, source: ServerRef? = null, landscape: Boolean = false,
    progress: Float? = null, watched: Boolean = false, onClick: () -> Unit,
) {
    var focused by remember { mutableStateOf(false) }
    val scale by animateFloatAsState(if (focused) 1.025f else 1f, label = "posterFocus")
    val palette = MaterialTheme.colorScheme
    val shape = RoundedCornerShape(6.dp)
    Column(modifier.fillMaxWidth().onFocusChanged { focused = it.hasFocus }.clickable(
        interactionSource = remember { MutableInteractionSource() }, indication = null, role = Role.Button, onClick = onClick,
    )) {
        Box(Modifier.fillMaxWidth().aspectRatio(if (landscape) 16f / 9f else 2f / 3f)
            .graphicsLayer { scaleX = scale; scaleY = scale }
            .shadow(if (focused) 10.dp else 3.dp, shape)
            .clip(shape).border(if (focused) 2.dp else 1.dp, if (focused) palette.primary else palette.onSurface.copy(alpha = 0.08f), shape)) {
            Poster(image, api, Modifier.fillMaxSize(), source)
            if (image == null) Text(title.take(1).uppercase(), modifier = Modifier.align(Alignment.Center), style = MaterialTheme.typography.headlineLarge, color = palette.onSurfaceVariant)
            if (watched) Icon(Icons.Default.Check, contentDescription = null, tint = palette.primary,
                modifier = Modifier.align(Alignment.TopEnd).padding(6.dp).background(Color.Black.copy(alpha = 0.65f), RoundedCornerShape(4.dp)).padding(4.dp).size(16.dp))
            if (progress != null) LinearProgressIndicator(progress = { progress.coerceIn(0f, 1f) },
                modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(4.dp), color = palette.primary, trackColor = Color.Black.copy(alpha = 0.6f))
        }
        Text(title, modifier = Modifier.padding(top = 8.dp), style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.SemiBold, color = if (focused) Color.White else palette.onSurface,
            maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (!subtitle.isNullOrBlank()) Text(subtitle, style = MaterialTheme.typography.bodySmall,
            color = palette.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}