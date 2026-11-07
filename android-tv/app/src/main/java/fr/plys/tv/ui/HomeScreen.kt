package fr.plys.tv.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Movie
import androidx.compose.material.icons.outlined.Tv
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.CatalogCount
import fr.plys.tv.data.CatalogKind
import fr.plys.tv.data.HomeItem
import fr.plys.tv.data.HomeResponse
import fr.plys.tv.data.ItemCard
import fr.plys.tv.data.Library

// Focused tiles grow and draw a border and a shadow: rows keep this room around them so nothing is clipped,
// and every element of the screen starts at the same horizontal gutter.
val FocusGutter = 16.dp
val RowFocusPadding = PaddingValues(horizontal = FocusGutter, vertical = FocusGutter)

fun kindLabel(kind: CatalogKind): Int = when (kind) {
    CatalogKind.MOVIES -> R.string.movies
    CatalogKind.SERIES -> R.string.series
    CatalogKind.OTHER -> R.string.other
}

fun kindIcon(kind: CatalogKind): ImageVector = when (kind) {
    CatalogKind.MOVIES -> Icons.Outlined.Movie
    CatalogKind.SERIES -> Icons.Outlined.Tv
    CatalogKind.OTHER -> Icons.Outlined.VideoLibrary
}

private fun recentLabel(kind: CatalogKind): Int = when (kind) {
    CatalogKind.MOVIES -> R.string.recent_movies
    CatalogKind.SERIES -> R.string.recent_series
    CatalogKind.OTHER -> R.string.recent_other
}

/**
 * Full-width rows stacked vertically, the TV convention: up and down change row, left and right browse it.
 * What the viewer was watching comes first, then the libraries and what was added lately, so the home stays
 * useful even before anything has been watched.
 */
@Composable
fun HomeScreen(
    data: HomeResponse, model: CatalogViewModel, libraries: List<Library>, catalogCounts: Map<CatalogKind, CatalogCount>,
    recent: Map<CatalogKind, List<ItemCard>> = emptyMap(),
) {
    val firstRail = data.continueWatching.ifEmpty { data.nextUp }
    LazyColumn(Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(8.dp), contentPadding = PaddingValues(top = 8.dp, bottom = 48.dp)) {
        if (data.unavailableServers > 0) item("partial") {
            Text(stringResource(R.string.partial_results, data.unavailableServers), color = Plys.Warning, style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.padding(horizontal = FocusGutter))
        }
        if (data.continueWatching.isNotEmpty()) item("continue") {
            HomeRail(R.string.continue_watching, data.continueWatching, model, requestInitial = true)
        }
        if (data.nextUp.isNotEmpty()) item("next") {
            HomeRail(R.string.next_up, data.nextUp, model, requestInitial = data.continueWatching.isEmpty())
        }
        item("libraries") {
            Column {
                RailTitle(R.string.libraries)
                Row(Modifier.padding(horizontal = FocusGutter, vertical = FocusGutter), horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                    CatalogKind.entries.forEach { kind ->
                        KindTile(kind, catalogCounts[kind], libraries.count { it.kind == kind.value },
                            Modifier.focusInitially(firstRail.isEmpty() && kind == CatalogKind.MOVIES)) { model.openKind(kind) }
                    }
                }
            }
        }
        CatalogKind.entries.forEach { kind ->
            val cards = recent[kind].orEmpty()
            if (cards.isNotEmpty()) item("recent-${kind.value}") {
                Column {
                    RailTitle(recentLabel(kind))
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(20.dp), contentPadding = RowFocusPadding) {
                        items(cards, key = { it.id }) { card ->
                            MediaCard(card, model.api, Modifier.width(if (card.kind == "video") 232.dp else 132.dp)) { model.openCard(card) }
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun RailTitle(title: Int, modifier: Modifier = Modifier) {
    Text(stringResource(title), style = MaterialTheme.typography.titleLarge, color = Plys.TextStrong,
        modifier = modifier.padding(horizontal = FocusGutter).padding(top = 8.dp))
}

/** A library kind in the colours of the web home: its icon, the number of titles and of libraries. */
@Composable
private fun KindTile(kind: CatalogKind, titles: CatalogCount?, libraries: Int, modifier: Modifier, onClick: () -> Unit) {
    val gradient = when (kind) { CatalogKind.MOVIES -> Plys.MoviesGradient; CatalogKind.SERIES -> Plys.SeriesGradient; CatalogKind.OTHER -> Plys.OtherGradient }
    PlysButton(onClick = onClick, modifier = modifier.width(232.dp).height(112.dp), shape = Plys.Card, contentPadding = PaddingValues(0.dp)) {
        Box(Modifier.fillMaxSize().background(gradient).padding(16.dp)) {
            Icon(kindIcon(kind), contentDescription = null, tint = LocalContentColor.current.copy(alpha = 0.85f),
                modifier = Modifier.align(Alignment.TopEnd).size(30.dp))
            Column(Modifier.align(Alignment.BottomStart), verticalArrangement = Arrangement.spacedBy(0.dp)) {
                Text(stringResource(kindLabel(kind)), style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold,
                    maxLines = 1, overflow = TextOverflow.Ellipsis)
                val titleLabel = titles?.let { pluralStringResource(R.plurals.title_count, it.total, it.total) }
                    ?: stringResource(R.string.title_count_pending)
                val secondary = LocalContentColor.current.copy(alpha = 0.75f)
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Text(if (titles?.partial == true) "≥ $titleLabel" else titleLabel,
                        style = MaterialTheme.typography.bodySmall, color = secondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    if (libraries > 0) Text(pluralStringResource(R.plurals.library_count, libraries, libraries),
                        style = MaterialTheme.typography.bodySmall, color = secondary, maxLines = 1)
                }
            }
        }
    }
}

@Composable
private fun HomeRail(title: Int, entries: List<HomeItem>, model: CatalogViewModel, requestInitial: Boolean, modifier: Modifier = Modifier) {
    Column(modifier) {
        RailTitle(title)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(20.dp), contentPadding = RowFocusPadding) {
            items(entries, key = { it.source.serverId + it.id }) { item ->
                val episode = episodeCode(item.seasonNumber, item.episodeNumber)
                val left = (item.durationSeconds - item.positionSeconds).takeIf { item.positionSeconds > 0 && item.durationSeconds > 0 }
                PosterTile(title = item.showTitle.ifBlank { item.title }, image = item.posterUrl, api = model.api, source = item.source,
                    modifier = Modifier.width(232.dp).focusInitially(requestInitial && item == entries.first()), landscape = true,
                    label = episode,
                    subtitle = listOfNotNull(
                        item.title.takeIf { item.showTitle.isNotBlank() },
                        left?.let { stringResource(R.string.time_left, durationLabel(it)) },
                        item.source.serverName.takeIf { it.isNotBlank() },
                    ).joinToString(" · "),
                    progress = if (item.durationSeconds > 0) (item.positionSeconds / item.durationSeconds).toFloat() else null,
                ) { model.openItem(item.id, item.source) }
            }
        }
    }
}

/** "S1 · E4", or null outside series. */
fun episodeCode(season: Int?, episode: Int?): String? = when {
    season != null && episode != null -> "S$season · E$episode"
    episode != null -> "E$episode"
    else -> null
}

/** A duration for people, e.g. "1 h 05" or "42 min". */
fun durationLabel(seconds: Double): String {
    val minutes = kotlin.math.max(1L, kotlin.math.round(seconds / 60).toLong())
    return if (minutes >= 60) "${minutes / 60} h ${(minutes % 60).toString().padStart(2, '0')}" else "$minutes min"
}
