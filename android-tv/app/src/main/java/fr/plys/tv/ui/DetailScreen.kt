package fr.plys.tv.ui

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.BringIntoViewSpec
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.layout
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.offset
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import fr.plys.tv.R
import fr.plys.tv.data.Ancestor
import fr.plys.tv.data.ChildItem
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.ServerRef
import kotlinx.coroutines.launch
import fr.plys.tv.ui.PlysButton as Button

/**
 * The web's item page adapted to a TV: the backdrop fills the screen behind the information, the play button
 * holds the focus, and seasons, episodes, cast and crew follow as rows reached by pressing down.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun DetailScreen(screen: Screen.Detail, model: CatalogViewModel) {
    val item = screen.item
    val source = screen.source
    val playable = screen.playable
    val list = rememberLazyListState()
    val scope = rememberCoroutineScope()
    var synopsis by remember { mutableStateOf(false) }
    // The play button takes the initial focus; focusing anything in the header brings the whole of it back in view.
    // Waits a frame so this scroll comes after, and replaces, the one bringing the focused element into view.
    val showTop = Modifier.onFocusChanged {
        if (it.hasFocus) scope.launch { androidx.compose.runtime.withFrameNanos { }; list.animateScrollToItem(0) }
    }
    val canPlay = item.playableVersion != null || item.playTarget != null
    // The play key plays the title wherever the focus is on the page, except on a season or episode tile, which plays that one.
    Box(Modifier.fillMaxSize().onPlayKey(if (canPlay) ({ model.play(item) }) else null)) {
        Backdrop(item.backdropUrl ?: item.thumbUrl, model, screen)
        CompositionLocalProvider(LocalBringIntoViewSpec provides MinimalScroll) {
            LazyColumn(state = list, modifier = Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(12.dp),
                contentPadding = PaddingValues(start = RailWidth + 24.dp - FocusGutter, end = 40.dp - FocusGutter, top = 36.dp, bottom = 48.dp)) {
                item("header") {
                    Row(Modifier.padding(horizontal = FocusGutter).then(showTop), horizontalArrangement = Arrangement.spacedBy(32.dp)) {
                        Poster(item.posterUrl ?: item.thumbUrl, model.api,
                            Modifier.width(172.dp).height(if (item.posterUrl == null && item.thumbUrl != null) 97.dp else 258.dp)
                                .shadow(20.dp, Plys.Card).clip(Plys.Card).border(1.dp, Plys.Hairline, Plys.Card), source)
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            Header(item, screen) { model.openItem(it.id, source) }
                            item.overview?.takeIf { it.isNotBlank() }?.let { Synopsis(it) { synopsis = true } }
                            // One line whatever the actions: once a title is started the play button turns into "Resume at…"
                            // and a restart button joins it, which squeezed the last buttons until their labels wrapped and
                            // the header grew. Apart from play, the actions (restart, watched, watchlist, next episode) are
                            // icons, like on the web, and the row scrolls sideways (following the focus) if it still runs out of room.
                            Row(Modifier.padding(top = 8.dp).outdent(FocusGutter).horizontalScroll(rememberScrollState()).padding(horizontal = FocusGutter),
                                horizontalArrangement = Arrangement.spacedBy(14.dp), verticalAlignment = Alignment.CenterVertically) {
                                if (canPlay) {
                                    val position = item.playTarget?.positionSeconds ?: item.positionSeconds
                                    Button(onClick = { model.play(item) }, primary = true, modifier = Modifier.focusInitially(),
                                        contentPadding = PaddingValues(horizontal = 22.dp, vertical = 10.dp)) {
                                        Icon(Icons.Outlined.PlayArrow, contentDescription = null, modifier = Modifier.size(24.dp), tint = LocalContentColor.current)
                                        Spacer(Modifier.width(8.dp))
                                        Text(playLabel(item, playable, position), style = MaterialTheme.typography.titleMedium, maxLines = 1, softWrap = false)
                                    }
                                    if (position > 0 && item.playableVersion != null) Action(R.string.restart, iconOnly = true) { model.play(item, restart = true) }
                                } else if (item.children.isEmpty()) Text(stringResource(R.string.unavailable), color = Plys.Warning)
                                if (canPlay || item.kind == "episode") Action(if (item.watched) R.string.mark_unwatched else R.string.mark_watched, iconOnly = true) { model.toggleWatched() }
                                if (item.canWatchlist) Action(if (item.inWatchlist) R.string.remove_from_watchlist else R.string.add_to_watchlist, iconOnly = true) { model.toggleWatchlist() }
                                item.next?.let { next -> Action(R.string.next_episode, iconOnly = true) { model.openItem(next.id, source) } }
                            }
                        }
                    }
                }
                item("tracks") { Box(Modifier.fillMaxWidth().padding(start = 172.dp + 32.dp).then(showTop)) { PlaybackSelectionControls(screen, model) } }
                if (item.children.isNotEmpty()) item("children") {
                    ChildrenRow(item, model, source, focusFirst = !canPlay)
                }
                if (item.cast.isNotEmpty()) item("cast") {
                    PeopleRow(R.string.cast, item.cast, model.api, source) { model.openPerson(item, "actors", it) }
                }
                val directors = item.directorPeople
                if (directors.isNotEmpty()) item("directors") {
                    PeopleRow(if (item.kind == "series") R.string.creators_title else R.string.directors_title, directors, model.api, source) {
                        model.openPerson(item, "creators", it)
                    }
                }
            }
        }
    }
    if (synopsis) SynopsisDialog(detailTitle(item), item.overview.orEmpty()) { synopsis = false }
}

/** The backdrop, faded towards the left where the text is and towards the bottom where the rows scroll. */
@Composable
private fun Backdrop(path: String?, model: CatalogViewModel, screen: Screen.Detail) {
    Box(Modifier.fillMaxSize()) {
        if (path != null) AsyncImage(model = model.api.posterUrl(path, screen.source), contentDescription = null,
            modifier = Modifier.fillMaxWidth().fillMaxHeight(0.85f).align(Alignment.TopEnd).alpha(0.5f), contentScale = ContentScale.Crop)
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0f to Plys.Surface950, 0.35f to Plys.Surface950.copy(alpha = 0.85f), 1f to Color.Transparent)))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.3f to Color.Transparent, 0.85f to Plys.Surface950)))
    }
}

@Composable
private fun Header(item: ItemDetail, screen: Screen.Detail, openAncestor: (Ancestor) -> Unit) {
    if (item.ancestors.isNotEmpty()) Breadcrumb(item.ancestors, openAncestor)
    Text(detailTitle(item), style = MaterialTheme.typography.displaySmall, color = Plys.TextStrong, maxLines = 2, overflow = TextOverflow.Ellipsis)
    item.originalTitle?.takeIf { it.isNotBlank() && it != item.title }?.let {
        Text(it, style = MaterialTheme.typography.bodyMedium, color = Plys.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
    val facts = listOfNotNull(
        episodeCode(item.seasonNumber.takeIf { item.kind == "episode" }, item.episodeNumber),
        item.year?.toString(),
        item.runtimeMinutes?.takeIf { it > 0 }?.let { durationLabel(it * 60.0) },
        item.rating?.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
        screen.source?.serverName?.takeIf { it.isNotBlank() },
        stringResource(R.string.watched).takeIf { item.watched },
    )
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
        if (facts.isNotEmpty()) Text(facts.joinToString("  ·  "), style = MaterialTheme.typography.bodyMedium, color = Plys.Text)
        item.genres.take(4).forEach { genre ->
            Text(genre, style = MaterialTheme.typography.labelMedium, color = Plys.Text, maxLines = 1,
                modifier = Modifier.border(1.dp, Color.White.copy(alpha = 0.18f), RoundedCornerShape(50)).padding(horizontal = 10.dp, vertical = 2.dp))
        }
    }
    item.tagline?.takeIf { it.isNotBlank() }?.let {
        Text(it, style = MaterialTheme.typography.bodyLarge, fontStyle = FontStyle.Italic, color = Plys.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
    VersionSummary(screen.playable, screen.selection)
}

/**
 * The series and season above an episode, or the series above a season, like the web's breadcrumb: each one is a
 * button, reached by pressing up from the actions, that opens that level (back returns here).
 */
@Composable
private fun Breadcrumb(ancestors: List<Ancestor>, open: (Ancestor) -> Unit) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        ancestors.forEachIndexed { index, ancestor ->
            if (index > 0) Text("›", style = MaterialTheme.typography.titleSmall, color = Plys.Muted)
            val label = if (ancestor.kind == "season" && ancestor.seasonNumber != null) stringResource(R.string.season, ancestor.seasonNumber) else ancestor.title
            Button(onClick = { open(ancestor) }, modifier = Modifier.weight(1f, fill = false), contentPadding = PaddingValues(horizontal = 12.dp, vertical = 4.dp)) {
                Text(label, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

/** "Play", "Resume at 12:04" or, on a series, the episode that will play. */
@Composable
private fun playLabel(item: ItemDetail, playable: ItemDetail, position: Double): String {
    val base = if (position > 0) stringResource(R.string.resume, clock(position)) else stringResource(R.string.play)
    if (item.playableVersion != null || playable.id == item.id) return base
    val code = episodeCode(playable.seasonNumber, playable.episodeNumber) ?: return base
    return "$base · $code"
}

@Composable
private fun ChildrenRow(item: ItemDetail, model: CatalogViewModel, source: ServerRef?, focusFirst: Boolean) {
    val title = when (item.kind) {
        "series" -> R.string.seasons
        "season" -> R.string.episodes
        else -> R.string.parts
    }
    Column {
        RailTitle(title)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(20.dp), contentPadding = RowFocusPadding) {
            items(item.children, key = { it.id }) { child ->
                val modifier = Modifier.focusInitially(focusFirst && child == item.children.first())
                if (child.kind == "season") PosterTile(
                    title = childLabel(child), image = child.posterUrl, api = model.api, source = source,
                    modifier = modifier.width(132.dp), watched = child.watched, unwatched = child.unwatchedCount,
                    onPlay = { model.playItem(child.id, source) },
                ) { model.openItem(child.id, source) }
                else {
                    val runtime = child.runtimeMinutes?.takeIf { it > 0 }
                    PosterTile(
                        title = child.title.ifBlank { childLabel(child) }, image = child.thumbUrl ?: child.posterUrl, api = model.api, source = source,
                        modifier = modifier.width(248.dp), landscape = true, watched = child.watched,
                        label = when (child.kind) {
                            "episode" -> child.episodeNumber?.let { "E$it" }
                            "part" -> child.partNumber?.let { stringResource(R.string.part, it) }
                            else -> null
                        },
                        subtitle = listOfNotNull(
                            runtime?.let { durationLabel(it * 60.0) },
                            child.overview?.takeIf { it.isNotBlank() },
                        ).joinToString(" · "),
                        progress = runtime?.let { (child.positionSeconds / (it * 60.0)).toFloat() }?.takeIf { child.positionSeconds > 0 },
                        onPlay = { model.playItem(child.id, source) },
                    ) { model.openItem(child.id, source) }
                }
            }
        }
    }
}

/**
 * Scrolls only as much as needed to show the focused element. TV's default keeps the focused element about a
 * third down the screen, which scrolled the top of the detail page away as soon as the play button took focus.
 */
@OptIn(ExperimentalFoundationApi::class)
private val MinimalScroll = object : BringIntoViewSpec {
    override fun calculateScrollDistance(offset: Float, size: Float, containerSize: Float): Float {
        val end = offset + size
        return when {
            offset >= 0 && end <= containerSize -> 0f
            offset < 0 || size > containerSize -> offset
            else -> end - containerSize
        }
    }
}

/**
 * Widens the element by [gutter] on both sides without taking more room, so a scrolling row, which clips its
 * content at its edges, leaves space for the slightly enlarged focused button at either end.
 */
private fun Modifier.outdent(gutter: Dp) = layout { measurable, constraints ->
    val extra = gutter.roundToPx()
    val placeable = measurable.measure(constraints.offset(horizontal = 2 * extra))
    layout((placeable.width - 2 * extra).coerceAtLeast(0), placeable.height) { placeable.place(-extra, 0) }
}

/** Seasons and episodes without metadata have no title: show their number instead. */
@Composable
fun detailTitle(item: ItemDetail): String = item.title.ifBlank {
    when {
        item.kind == "season" && item.seasonNumber != null -> stringResource(R.string.season, item.seasonNumber)
        item.kind == "episode" && item.episodeNumber != null -> stringResource(R.string.episode, item.episodeNumber)
        else -> ""
    }
}

@Composable
private fun childLabel(item: ChildItem): String {
    val label = when (item.kind) {
        "season" -> stringResource(R.string.season, item.seasonNumber ?: 0)
        "episode" -> stringResource(R.string.episode, item.episodeNumber ?: 0)
        "part" -> stringResource(R.string.part, item.partNumber ?: 0)
        else -> ""
    }
    return listOf(label, item.title).filter { it.isNotBlank() }.distinct().joinToString(" · ")
}
