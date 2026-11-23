package fr.plys.tv.ui

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.Icon
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
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.PlysApi
import java.util.Locale
import kotlin.math.roundToInt
import fr.plys.tv.data.ServerRef

/** Focused tiles grow by this factor: rows and grids keep [FocusGutter] around them so nothing is clipped. */
private const val FOCUSED_SCALE = 1.08f

/**
 * A poster (2:3) or a thumbnail (16:9) with its title, mirroring the web's `PosterCard`: rounded artwork, a
 * watched check or the count of episodes left, and a resume bar. Focus lifts the tile and rings it with the accent.
 * [onPlay], when set, runs on the remote's play key while the tile has the focus.
 */
@Composable
fun PosterTile(
    title: String, image: String?, api: PlysApi, modifier: Modifier = Modifier,
    subtitle: String? = null, rating: Double? = null, source: ServerRef? = null, landscape: Boolean = false,
    progress: Float? = null, watched: Boolean = false, unwatched: Int? = null, label: String? = null,
    onPlay: (() -> Unit)? = null, onClick: () -> Unit,
) {
    var focused by remember { mutableStateOf(false) }
    val scale by animateFloatAsState(if (focused) FOCUSED_SCALE else 1f, label = "posterFocus")
    val shape = Plys.Card
    Column(modifier.fillMaxWidth().onFocusChanged { focused = it.hasFocus }.onPlayKey(onPlay).clickable(
        interactionSource = remember { MutableInteractionSource() }, indication = null, role = Role.Button, onClick = onClick,
    )) {
        Box(Modifier.fillMaxWidth().aspectRatio(if (landscape) 16f / 9f else 2f / 3f)
            .graphicsLayer { scaleX = scale; scaleY = scale }
            .shadow(if (focused) 18.dp else 4.dp, shape, ambientColor = if (focused) Plys.Accent500 else Color.Black, spotColor = if (focused) Plys.Accent500 else Color.Black)
            .clip(shape)
            .border(if (focused) 3.dp else 1.dp, if (focused) Plys.Accent300 else Plys.Hairline, shape)) {
            if (image != null) Poster(image, api, Modifier.fillMaxSize(), source)
            else Box(Modifier.fillMaxSize().background(Brush.linearGradient(listOf(Plys.Surface700, Plys.Surface900)))) {
                Text(title.trim().take(1).uppercase().ifEmpty { "?" }, modifier = Modifier.align(Alignment.Center),
                    style = MaterialTheme.typography.displaySmall, color = Color.White.copy(alpha = 0.2f))
            }
            if (label != null) Text(label, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.Bold, color = Color.White,
                modifier = Modifier.align(Alignment.BottomStart).padding(8.dp)
                    .background(Color.Black.copy(alpha = 0.65f), RoundedCornerShape(6.dp)).padding(horizontal = 6.dp, vertical = 2.dp))
            when {
                unwatched != null && unwatched > 0 -> Text(unwatched.toString(), style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.Bold,
                    color = Plys.Surface950, modifier = Modifier.align(Alignment.TopEnd).padding(8.dp)
                        .background(Plys.Accent500, RoundedCornerShape(6.dp)).padding(horizontal = 7.dp, vertical = 1.dp))
                watched -> Icon(Icons.Default.Check, contentDescription = null, tint = Plys.Accent300,
                    modifier = Modifier.align(Alignment.TopEnd).padding(8.dp).size(26.dp).background(Color.Black.copy(alpha = 0.6f), CircleShape).padding(4.dp))
            }
            if (progress != null && progress > 0f) Box(Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(5.dp).background(Color.Black.copy(alpha = 0.6f))) {
                Box(Modifier.fillMaxWidth(progress.coerceIn(0f, 1f)).height(5.dp).background(Plys.Accent400))
            }
        }
        Text(title, modifier = Modifier.padding(top = 10.dp), style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.SemiBold, color = if (focused) Plys.TextStrong else Plys.Text,
            maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (!subtitle.isNullOrBlank() || rating != null && rating > 0) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                if (!subtitle.isNullOrBlank()) Text(subtitle, style = MaterialTheme.typography.bodySmall,
                    color = if (focused) Plys.Muted else Plys.Subtle, maxLines = 1, overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f))
                if (rating != null && rating > 0) {
                    if (!subtitle.isNullOrBlank()) Spacer(Modifier.width(8.dp))
                    TmdbRating(rating)
                }
            }
        }
    }
}

@Composable
fun TmdbRating(rating: Double, modifier: Modifier = Modifier) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        Image(painterResource(R.drawable.tmdb_logo_short), contentDescription = "TMDB", modifier = Modifier.width(20.dp).height(9.dp))
        Spacer(Modifier.width(5.dp))
        Text("${(rating.coerceIn(0.0, 10.0) * 10).roundToInt()}%", style = MaterialTheme.typography.bodySmall, color = Plys.Text)
    }
}

/** IMDb's public rating out of 10, behind a yellow IMDb mark like the web's. */
@Composable
fun ImdbRating(rating: Double, modifier: Modifier = Modifier) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        Text("IMDb", style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Black, color = Color.Black,
            modifier = Modifier.background(Color(0xFFF5C518), RoundedCornerShape(2.dp)).padding(horizontal = 4.dp))
        Spacer(Modifier.width(5.dp))
        Text(String.format(Locale.getDefault(), "%.1f", rating.coerceIn(0.0, 10.0)), style = MaterialTheme.typography.bodySmall, color = Plys.Text)
    }
}
