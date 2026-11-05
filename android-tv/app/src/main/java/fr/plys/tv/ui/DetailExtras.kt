package fr.plys.tv.ui

import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.CastMember
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.MediaTrack
import fr.plys.tv.data.PlaybackSelection
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.ServerRef

fun MediaTrack.trackLabel(): String = listOfNotNull(
    language.takeIf { it.isNotBlank() }, title.takeIf { it.isNotBlank() },
    "#${index + 1}".takeIf { language.isBlank() && title.isBlank() },
    codec.uppercase().takeIf { it.isNotBlank() }, "$channels ch".takeIf { channels > 0 },
).joinToString(" · ")

@Composable
fun PlaybackSelectionControls(item: ItemDetail, selection: PlaybackSelection, select: (PlaybackSelection) -> Unit) {
    val chosen = selection.normalized(item)
    val version = chosen.version(item) ?: return
    var menu by remember { mutableStateOf("") }
    LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.padding(vertical = 4.dp)) {
        if (item.versions.size > 1) item {
            PlysButton(onClick = { menu = "version" }) { Text("${stringResource(R.string.version)} : ${version.label}", maxLines = 1) }
        }
        if (version.audio.size > 1) item {
            PlysButton(onClick = { menu = "audio" }) { Text("${stringResource(R.string.audio)} : ${version.audio.firstOrNull { it.index == chosen.audioIndex }?.trackLabel().orEmpty()}", maxLines = 1) }
        }
        if (version.subtitles.isNotEmpty()) item {
            PlysButton(onClick = { menu = "subtitles" }) { Text("${stringResource(R.string.subtitles)} : ${version.subtitles.firstOrNull { it.index == chosen.subtitleIndex }?.trackLabel() ?: stringResource(R.string.no_subtitles)}", maxLines = 1) }
        }
    }
    when (menu) {
        "version" -> ChoiceDialog(stringResource(R.string.version), { menu = "" }) {
            items(item.versions, key = { it.id }) { candidate ->
                PlysButton(onClick = { select(PlaybackSelection(candidate.id).normalized(item)); menu = "" }, enabled = candidate.error == null) { Text(candidate.label) }
            }
        }
        "audio" -> ChoiceDialog(stringResource(R.string.audio), { menu = "" }) {
            items(version.audio, key = { it.index }) { track ->
                PlysButton(onClick = { select(chosen.copy(audioIndex = track.index)); menu = "" }) { Text(track.trackLabel()) }
            }
        }
        "subtitles" -> ChoiceDialog(stringResource(R.string.subtitles), { menu = "" }) {
            item { PlysButton(onClick = { select(chosen.copy(subtitleIndex = null)); menu = "" }) { Text(stringResource(R.string.no_subtitles)) } }
            items(version.subtitles, key = { it.index }) { track ->
                PlysButton(onClick = { select(chosen.copy(subtitleIndex = track.index)); menu = "" }) { Text(track.trackLabel()) }
            }
        }
    }
}

@Composable
fun CastRow(cast: List<CastMember>, api: PlysApi, source: ServerRef?) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(stringResource(R.string.cast), style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.onSurface)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(16.dp), modifier = Modifier.padding(4.dp)) {
            items(cast) { actor ->
                var focused by remember { mutableStateOf(false) }
                Column(Modifier.width(104.dp).onFocusChanged { focused = it.hasFocus }.focusable(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Box(Modifier.size(72.dp).clip(RoundedCornerShape(6.dp))
                        .border(if (focused) 2.dp else 0.dp, if (focused) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surface, RoundedCornerShape(6.dp))) {
                        Poster(actor.photoUrl, api, Modifier.size(72.dp), source)
                        if (actor.photoUrl == null) Text(actor.name.take(1), modifier = Modifier.align(Alignment.Center), style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    Text(actor.name, modifier = Modifier.height(40.dp), style = MaterialTheme.typography.bodyMedium,
                        maxLines = 2, overflow = TextOverflow.Ellipsis, color = MaterialTheme.colorScheme.onSurface)
                    if (actor.character.isNotBlank()) Text(actor.character, style = MaterialTheme.typography.bodySmall, maxLines = 1, overflow = TextOverflow.Ellipsis, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
    }
}