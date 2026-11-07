package fr.plys.tv.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.gestures.BringIntoViewSpec
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.focus.onFocusChanged
import kotlinx.coroutines.launch
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import fr.plys.tv.ui.PlysButton as Button
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.automirrored.outlined.Logout
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Pause
import androidx.compose.material.icons.outlined.Done
import androidx.compose.material.icons.outlined.RemoveDone
import androidx.compose.material.icons.outlined.Replay
import androidx.compose.material3.Icon
import fr.plys.tv.R
import fr.plys.tv.data.ItemCard
import fr.plys.tv.data.ChildItem
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.ServerRef

@Composable
fun CatalogApp(model: CatalogViewModel, state: CatalogState, modifier: Modifier = Modifier) {
    BackHandler(model.canGoBack || state.loading) { model.back() }
    val compact = state.screen is Screen.Catalog || state.screen is Screen.Detail
    Column(modifier.padding(horizontal = 48.dp, vertical = if (compact) 12.dp else 24.dp)) {
        if (!compact) {
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Text("Plys TV", modifier = Modifier.weight(1f))
            if (state.screen != Screen.Login && state.screen !is Screen.Home) Action(R.string.home) { model.home() }
            if (model.canGoBack) Action(R.string.back) { model.back() }
            if (state.screen != Screen.Login) Action(R.string.sign_out, !state.loading) { model.logout() }
        }
        Spacer(Modifier.height(16.dp))
        }
        state.error?.let {
            Text(LocalContext.current.errorMessage(it), color = androidx.tv.material3.MaterialTheme.colorScheme.error)
            if (state.canRetry) Action(R.string.retry, !state.loading) { model.retry() }
            Spacer(Modifier.height(12.dp))
        }
        if (state.loading) {
            CircularProgressIndicator(color = androidx.tv.material3.MaterialTheme.colorScheme.primary)
            Text(stringResource(R.string.loading))
        }
        if (state.screen == Screen.Login) {
            Login(model, state.loading)
        } else {
            when (val screen = state.screen) {
                Screen.Login -> Unit
                is Screen.Home -> key("home") { HomeScreen(screen.data, model, state.libraries, state.catalogCounts) }
                is Screen.Libraries -> {
                    Text(stringResource(R.string.libraries))
                    if (screen.items.isEmpty()) Text(stringResource(R.string.empty))
                    key("libraries") {
                        LazyColumn(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            items(screen.items, key = { it.id }) { library ->
                                Button(
                                    onClick = { model.openLibrary(library) },
                                    modifier = Modifier.fillMaxWidth().then(
                                        Modifier.focusInitially(library == screen.items.first()),
                                    ),
                                ) { Text(library.name) }
                            }
                        }
                    }
                }
                is Screen.Catalog -> key(screen.query) { CatalogScreen(screen, model, state.libraries) }
                is Screen.Detail -> key(screen.source?.serverId, screen.item.id) { Detail(screen.item, model, screen.source, screen.sources, screen.selection, screen.target) }
            }
        }
    }
}

@Composable
private fun Login(model: CatalogViewModel, loading: Boolean) {
    var server by rememberSaveable { mutableStateOf(model.api.savedServer) }
    var username by rememberSaveable { mutableStateOf("") }
    // Do not put credentials into saved instance state.
    var password by remember { mutableStateOf("") }
    Column(
        Modifier.width(520.dp).verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        OutlinedTextField(server, { server = it }, Modifier.fillMaxWidth().focusInitially(!loading),
            label = { androidx.compose.material3.Text(stringResource(R.string.server)) }, singleLine = true, enabled = !loading,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri))
        OutlinedTextField(username, { username = it }, Modifier.fillMaxWidth(),
            label = { androidx.compose.material3.Text(stringResource(R.string.username)) }, singleLine = true, enabled = !loading)
        OutlinedTextField(password, { password = it }, Modifier.fillMaxWidth(),
            label = { androidx.compose.material3.Text(stringResource(R.string.password)) }, singleLine = true, enabled = !loading,
            visualTransformation = PasswordVisualTransformation(),
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password))
        Text(stringResource(R.string.http_warning))
        Action(R.string.sign_in, !loading && server.isNotBlank() && username.isNotBlank() && password.isNotEmpty()) {
            model.login(server, username.trim(), password)
            password = ""
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun Detail(item: ItemDetail, model: CatalogViewModel, source: ServerRef?, sources: List<ServerRef>, selection: fr.plys.tv.data.PlaybackSelection, target: ItemDetail?) {
    val playable = target ?: item
    val list = rememberLazyListState()
    val scope = rememberCoroutineScope()
    var synopsis by remember { mutableStateOf(false) }
    // The play button takes the initial focus; focusing anything above it brings the whole header back in view.
    // Waits a frame so this scroll comes after, and replaces, the one bringing the focused element into view.
    val showTop = Modifier.onFocusChanged {
        if (it.hasFocus) scope.launch { androidx.compose.runtime.withFrameNanos { }; list.animateScrollToItem(0) }
    }
    val canPlay = item.playableVersion != null || item.playTarget != null
    CompositionLocalProvider(LocalBringIntoViewSpec provides MinimalScroll) {
    LazyColumn(state = list, verticalArrangement = Arrangement.spacedBy(12.dp), contentPadding = PaddingValues(vertical = FocusGutter)) {
        item("header") {
            Row(Modifier.padding(horizontal = FocusGutter).then(showTop), horizontalArrangement = Arrangement.spacedBy(24.dp)) {
                Poster(item.posterUrl, model.api, Modifier.size(140.dp, 210.dp), source)
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(detailTitle(item), style = androidx.tv.material3.MaterialTheme.typography.headlineMedium, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    val facts = listOfNotNull(item.year?.toString(), source?.serverName?.takeIf { it.isNotBlank() },
                        stringResource(R.string.watched).takeIf { item.watched })
                    if (facts.isNotEmpty()) Text(facts.joinToString(" · "), color = androidx.tv.material3.MaterialTheme.colorScheme.onSurfaceVariant)
                    VersionSummary(playable, selection)
                    if (sources.size > 1) Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        sources.forEach { available -> Button(onClick = { model.selectSource(available) }) { Text(available.serverName) } }
                    }
                    item.overview?.takeIf { it.isNotBlank() }?.let { Synopsis(it) { synopsis = true } }
                }
            }
        }
        item("tracks") { Box(showTop) { PlaybackSelectionControls(playable, selection, model::selectPlayback) } }
        item("actions") {
            Row(Modifier.padding(horizontal = FocusGutter), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                if (canPlay) {
                    val position = item.playTarget?.positionSeconds ?: item.positionSeconds
                    Button(onClick = { model.play(item) }, modifier = Modifier.focusInitially()) {
                        Icon(Icons.Outlined.PlayArrow, contentDescription = null, modifier = Modifier.size(18.dp), tint = androidx.tv.material3.LocalContentColor.current)
                        Spacer(Modifier.width(8.dp))
                        Text(if (position > 0) stringResource(R.string.resume, clock(position)) else stringResource(R.string.play))
                    }
                    if (position > 0 && item.playableVersion != null) Action(R.string.restart) { model.play(item, restart = true) }
                } else if (item.children.isEmpty()) Text(stringResource(R.string.unavailable))
                if (item.kind == "episode") Action(if (item.watched) R.string.mark_unwatched else R.string.mark_watched) { model.toggleWatched() }
            }
        }
        if (item.cast.isNotEmpty()) item("cast") { PeopleRow(R.string.cast, item.cast, model.api, source) }
        val directors = item.directorPeople
        if (directors.isNotEmpty()) item("directors") {
            PeopleRow(if (item.kind == "series") R.string.creators_title else R.string.directors_title, directors, model.api, source)
        }
        items(item.children, key = { it.id }) { child ->
            Button(
                onClick = { model.openItem(child.id, source) },
                modifier = Modifier.padding(horizontal = FocusGutter).fillMaxWidth().then(
                    if (!canPlay && child == item.children.first()) Modifier.focusInitially() else Modifier,
                ),
            ) {
                Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                    Poster(child.thumbUrl ?: child.posterUrl, model.api, Modifier.size(96.dp, 54.dp), source)
                    Column {
                        Text(childLabel(child))
                        if (child.watched) Text(stringResource(R.string.watched))
                    }
                }
            }
        }
    }
    }
    if (synopsis) SynopsisDialog(detailTitle(item), item.overview.orEmpty()) { synopsis = false }
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
    return listOf(label, item.title).filter { it.isNotBlank() }.joinToString(" · ")
}

@Composable
fun MediaCard(item: ItemCard, api: PlysApi, modifier: Modifier, onClick: () -> Unit) {
    PosterTile(title = item.title, image = item.posterUrl, api = api, modifier = modifier,
        subtitle = listOfNotNull(item.year?.toString(), item.sources.firstOrNull()?.serverName?.takeIf { it.isNotBlank() }).joinToString(" · "),
        landscape = item.kind == "video", watched = item.watched, onClick = onClick)
}

@Composable
fun Poster(path: String?, api: PlysApi, modifier: Modifier, source: ServerRef? = null) {
    Box(modifier.background(androidx.tv.material3.MaterialTheme.colorScheme.surfaceVariant)) {
        if (path != null) AsyncImage(
            model = api.posterUrl(path, source), contentDescription = null,
            modifier = Modifier.fillMaxSize(), contentScale = ContentScale.Fit,
        )
    }
}

@Composable
fun Action(label: Int, enabled: Boolean = true, onClick: () -> Unit) {
    val icon = when (label) {
        R.string.home -> Icons.Outlined.Home
        R.string.back -> Icons.AutoMirrored.Outlined.ArrowBack
        R.string.sign_out -> Icons.AutoMirrored.Outlined.Logout
        R.string.retry, R.string.reset -> Icons.Outlined.Refresh
        R.string.apply -> Icons.Outlined.Check
        R.string.cancel -> Icons.Outlined.Close
        R.string.play -> Icons.Outlined.PlayArrow
        R.string.pause -> Icons.Outlined.Pause
        R.string.restart -> Icons.Outlined.Replay
        R.string.mark_watched -> Icons.Outlined.Done
        R.string.mark_unwatched -> Icons.Outlined.RemoveDone
        else -> null
    }
    Button(onClick = onClick, enabled = enabled) {
        if (icon != null) { Icon(icon, contentDescription = null, modifier = Modifier.size(18.dp), tint = androidx.tv.material3.LocalContentColor.current); Spacer(Modifier.width(8.dp)) }
        Text(stringResource(label))
    }
}

@Composable
fun Modifier.focusInitially(enabled: Boolean = true): Modifier {
    val requester = remember { FocusRequester() }
    var requested by remember { mutableStateOf(false) }
    return focusRequester(requester).onGloballyPositioned {
        if (enabled && !requested) {
            requested = true
            requester.requestFocus()
        }
    }
}
