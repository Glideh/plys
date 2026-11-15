package fr.plys.tv.ui

import androidx.activity.compose.BackHandler
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
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Movie
import androidx.compose.material.icons.outlined.Tv
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
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
import fr.plys.tv.data.MIN_SEARCH_LENGTH
import fr.plys.tv.data.SearchGroup

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
 * The search field comes first; from [MIN_SEARCH_LENGTH] characters one row per kind of matches replaces the
 * other rows. Otherwise what the viewer was watching comes first, then the libraries and what was added lately,
 * so the home stays useful even before anything has been watched.
 */
@Composable
fun HomeScreen(
    data: HomeResponse, model: CatalogViewModel, libraries: List<Library>, catalogCounts: Map<CatalogKind, CatalogCount>,
    recent: Map<CatalogKind, List<ItemCard>> = emptyMap(), search: SearchState = SearchState(), searchFocusRequested: Boolean = false,
) {
    // Once the viewer has typed, or came for the search, rows that come and go must not take the focus from the field.
    var typed by remember { mutableStateOf(searchFocusRequested) }
    var editSearch by remember { mutableStateOf(false) }
    val listState = rememberLazyListState()
    LaunchedEffect(searchFocusRequested) {
        if (searchFocusRequested) {
            typed = true
            // The field is the first row: bring it back into view so it can take the focus.
            listState.scrollToItem(0)
            editSearch = true
            model.searchFocusHandled()
        }
    }
    BackHandler(search.query.isNotEmpty()) { model.clearSearch() }
    val firstRail = data.continueWatching.ifEmpty { data.nextUp }
    LazyColumn(Modifier.fillMaxSize(), state = listState, verticalArrangement = Arrangement.spacedBy(8.dp), contentPadding = PaddingValues(top = 8.dp, bottom = 48.dp)) {
        item("search") {
            Column(Modifier.padding(horizontal = FocusGutter, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                SearchField(search.query, { typed = true; model.search(it) }, stringResource(R.string.search_everything),
                    Modifier.fillMaxWidth(0.6f), height = 48.dp, startEditing = editSearch) { editSearch = false }
                if (search.term.isNotEmpty() && !search.active) Text(stringResource(R.string.search_hint, MIN_SEARCH_LENGTH),
                    style = MaterialTheme.typography.bodySmall, color = Plys.Muted)
            }
        }
        if (search.active) {
            searchResults(search, model, requestInitial = !typed)
            return@LazyColumn
        }
        if (data.unavailableServers > 0) item("partial") {
            Text(partialResults(data.unavailableServers, data.unavailableServerNames), color = Plys.Warning, style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.padding(horizontal = FocusGutter))
        }
        if (data.continueWatching.isNotEmpty()) item("continue") {
            HomeRail(R.string.continue_watching, data.continueWatching, model, requestInitial = !typed)
        }
        if (data.nextUp.isNotEmpty()) item("next") {
            HomeRail(R.string.next_up, data.nextUp, model, requestInitial = !typed && data.continueWatching.isEmpty())
        }
        item("libraries") {
            Column {
                RailTitle(R.string.libraries)
                Row(Modifier.padding(horizontal = FocusGutter, vertical = FocusGutter), horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                    CatalogKind.entries.forEach { kind ->
                        KindTile(kind, catalogCounts[kind], libraries.count { it.kind == kind.value },
                            Modifier.focusInitially(!typed && firstRail.isEmpty() && kind == CatalogKind.MOVIES)) { model.openKind(kind) }
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
                            MediaCard(card, model.api, Modifier.width(if (card.kind == "video") 232.dp else 132.dp), onPlay = { model.playCard(card) }) { model.openCard(card) }
                        }
                    }
                }
            }
        }
    }
}

/** One row per kind with matches; the first match takes the focus when coming back from a result. */
private fun androidx.compose.foundation.lazy.LazyListScope.searchResults(search: SearchState, model: CatalogViewModel, requestInitial: Boolean) {
    val result = search.result
    val groups = result?.groups.orEmpty()
    val unavailable = result?.unavailableServers ?: 0
    if (unavailable > 0) item("search-partial") {
        Text(partialResults(unavailable, result?.unavailableServerNames.orEmpty()), color = Plys.Warning, style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.padding(horizontal = FocusGutter))
    }
    search.error?.let { error -> item("search-error") {
        ErrorBanner(LocalContext.current.errorMessage(error), { model.search(search.query, force = true) }, Modifier.padding(horizontal = FocusGutter))
    } }
    if (search.loading && groups.isEmpty()) item("search-loading") {
        Box(Modifier.fillMaxWidth().padding(24.dp), contentAlignment = Alignment.Center) {
            CircularProgressIndicator(Modifier.size(32.dp), color = Plys.Accent400, strokeWidth = 3.dp)
        }
    }
    if (result != null && groups.isEmpty() && !search.loading && search.error == null) item("search-empty") {
        EmptyState(stringResource(R.string.no_results), Modifier.padding(horizontal = FocusGutter))
    }
    groups.forEach { group ->
        item("search-${group.kind}") { SearchRow(group, model, requestInitial && group == groups.first()) }
    }
}

@Composable
private fun SearchRow(group: SearchGroup, model: CatalogViewModel, requestInitial: Boolean) {
    val kind = group.catalogKind ?: return
    Column {
        RailTitle("${stringResource(kindLabel(kind))} · ${pluralStringResource(R.plurals.title_count, group.total, group.total)}")
        LazyRow(horizontalArrangement = Arrangement.spacedBy(20.dp), contentPadding = RowFocusPadding, verticalAlignment = Alignment.CenterVertically) {
            items(group.items, key = { it.id }) { card ->
                MediaCard(card, model.api, Modifier.width(if (card.kind == "video") 232.dp else 132.dp)
                    .focusInitially(requestInitial && card == group.items.first()), onPlay = { model.playCard(card) }) { model.openCard(card) }
            }
            if (group.total > group.items.size) item("all") {
                PlysButton(onClick = { model.openSearchKind(kind) }) { Text(stringResource(R.string.see_all)) }
            }
        }
    }
}

@Composable
fun RailTitle(title: Int, modifier: Modifier = Modifier) = RailTitle(stringResource(title), modifier)

@Composable
fun RailTitle(title: String, modifier: Modifier = Modifier) {
    Text(title, style = MaterialTheme.typography.titleLarge, color = Plys.TextStrong,
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
                    onPlay = { model.playItem(item.id, item.source) },
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
