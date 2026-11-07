package fr.plys.tv.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import fr.plys.tv.ui.PlysButton as Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.CatalogKind
import fr.plys.tv.data.CatalogCount
import fr.plys.tv.data.HomeItem
import fr.plys.tv.data.HomeResponse
import fr.plys.tv.data.Library
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Movie
import androidx.compose.material.icons.outlined.Tv
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material3.Icon
import androidx.compose.ui.res.pluralStringResource

// Focused tiles grow and draw a border and a shadow: rows keep this room around them so nothing is clipped,
// and every element of the screen starts at the same horizontal gutter.
val FocusGutter = 12.dp
val RowFocusPadding = PaddingValues(horizontal = FocusGutter, vertical = FocusGutter)

fun kindLabel(kind: CatalogKind): Int = when (kind) {
    CatalogKind.MOVIES -> R.string.movies
    CatalogKind.SERIES -> R.string.series
    CatalogKind.OTHER -> R.string.other
}

@Composable
fun HomeScreen(data: HomeResponse, model: CatalogViewModel, libraries: List<Library>, catalogCounts: Map<CatalogKind, CatalogCount>) {
    val firstRail = data.continueWatching.ifEmpty { data.nextUp }
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        val sideBySide = maxWidth >= 640.dp && data.continueWatching.isNotEmpty() && data.nextUp.isNotEmpty()
        LazyColumn(verticalArrangement = Arrangement.spacedBy(32.dp), contentPadding = PaddingValues(bottom = FocusGutter)) {
        if (data.unavailableServers > 0) item { Text(stringResource(R.string.partial_results, data.unavailableServers), color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = FocusGutter)) }
        if (sideBySide) item {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(24.dp)) {
                HomeRail(R.string.continue_watching, data.continueWatching, model, true, Modifier.weight(1f))
                HomeRail(R.string.next_up, data.nextUp, model, false, Modifier.weight(1f))
            }
        }
        if (!sideBySide && data.continueWatching.isNotEmpty()) item {
            HomeRail(R.string.continue_watching, data.continueWatching, model, true)
        }
        if (!sideBySide && data.nextUp.isNotEmpty()) item {
            HomeRail(R.string.next_up, data.nextUp, model, data.continueWatching.isEmpty())
        }
        item {
            Row(Modifier.padding(horizontal = FocusGutter, vertical = FocusGutter), horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                CatalogKind.entries.forEach { kind ->
                    Button(onClick = { model.openKind(kind) }, modifier = Modifier.width(204.dp).height(80.dp).focusInitially(firstRail.isEmpty() && kind == CatalogKind.MOVIES)) {
                        Icon(when (kind) { CatalogKind.MOVIES -> Icons.Outlined.Movie; CatalogKind.SERIES -> Icons.Outlined.Tv; CatalogKind.OTHER -> Icons.Outlined.VideoLibrary },
                            contentDescription = null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(28.dp))
                        Column(Modifier.padding(start = 16.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            Text(stringResource(kindLabel(kind)), style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            val titles = catalogCounts[kind]
                            val titleLabel = titles?.let { pluralStringResource(R.plurals.title_count, it.total, it.total) }
                                ?: stringResource(R.string.title_count_pending)
                            Text(if (titles?.partial == true) "≥ $titleLabel" else titleLabel,
                                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            val count = libraries.count { it.kind == kind.value }
                            if (count > 0) Text(pluralStringResource(R.plurals.library_count, count, count), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }
        }
    }
    }
}

@Composable
private fun HomeRail(title: Int, entries: List<HomeItem>, model: CatalogViewModel, requestInitial: Boolean, modifier: Modifier = Modifier) {
    Column(modifier) {
        Text(stringResource(title), style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.onBackground,
            modifier = Modifier.padding(horizontal = FocusGutter))
        LazyRow(horizontalArrangement = Arrangement.spacedBy(16.dp), contentPadding = RowFocusPadding) {
            items(entries, key = { it.source.serverId + it.id }) { item ->
                PosterTile(title = item.showTitle.ifBlank { item.title }, image = item.posterUrl, api = model.api, source = item.source,
                    modifier = Modifier.width(144.dp).focusInitially(requestInitial && item == entries.first()), landscape = true,
                    subtitle = listOfNotNull(item.title.takeIf { item.showTitle.isNotBlank() }, item.source.serverName.takeIf { it.isNotBlank() }, clock(item.positionSeconds).takeIf { item.positionSeconds > 0 }).joinToString(" · "),
                    progress = if (item.durationSeconds > 0) (item.positionSeconds / item.durationSeconds).toFloat() else null,
                ) { model.openItem(item.id, item.source) }
            }
        }
    }
}