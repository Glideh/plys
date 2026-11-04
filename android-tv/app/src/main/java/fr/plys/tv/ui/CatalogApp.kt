package fr.plys.tv.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import fr.plys.tv.R
import fr.plys.tv.data.ItemCard
import fr.plys.tv.data.ChildItem
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.ServerRef
import kotlin.math.max

@Composable
fun CatalogApp(model: CatalogViewModel, state: CatalogState, modifier: Modifier = Modifier) {
    BackHandler(model.canGoBack || state.loading) { model.back() }
    Column(modifier.padding(horizontal = 48.dp, vertical = 24.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Text("Plys TV", modifier = Modifier.weight(1f))
            if (state.screen != Screen.Login && state.screen !is Screen.Home) Action(R.string.home) { model.home() }
            if (model.canGoBack) Action(R.string.back) { model.back() }
            if (state.screen != Screen.Login) Action(R.string.sign_out, !state.loading) { model.logout() }
        }
        Spacer(Modifier.height(16.dp))
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
                is Screen.Home -> key("home") { HomeScreen(screen.data, model) }
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
                is Screen.Catalog -> key(screen.query.kind, screen.query.libraryId, screen.page.offset) {
                    Text(stringResource(kindLabel(screen.query.kind)), style = androidx.tv.material3.MaterialTheme.typography.headlineMedium)
                    CatalogToolbar(screen, model, state.libraries)
                    val page = screen.page
                    if (page.unavailableServers > 0) Text(stringResource(R.string.partial_results, page.unavailableServers))
                    Row(horizontalArrangement = Arrangement.spacedBy(16.dp), modifier = Modifier.padding(vertical = 12.dp)) {
                        Action(R.string.previous_page, page.offset > 0) { model.page(max(0, page.offset - page.limit)) }
                        Text(stringResource(R.string.page, page.offset / page.limit + 1, max(1, (page.total + page.limit - 1) / page.limit)))
                        Action(R.string.next_page, page.offset + page.limit < page.total) { model.page(page.offset + page.limit) }
                    }
                    if (page.items.isEmpty()) Text(stringResource(R.string.empty))
                    val focusId = screen.focusedId ?: page.items.firstOrNull()?.id
                    val gridState = rememberLazyGridState(initialFirstVisibleItemIndex = page.items.indexOfFirst { it.id == focusId }.coerceAtLeast(0))
                    LazyVerticalGrid(
                        state = gridState,
                        columns = GridCells.Adaptive(160.dp),
                        horizontalArrangement = Arrangement.spacedBy(16.dp),
                        verticalArrangement = Arrangement.spacedBy(16.dp),
                    ) {
                        items(page.items, key = { it.id }) { item ->
                            MediaCard(item, model.api,
                                Modifier.focusInitially(item.id == focusId).onFocusChanged { if (it.hasFocus) model.rememberFocus(item.id) },
                            ) { model.openCard(item) }
                        }
                    }
                }
                is Screen.Detail -> key(screen.source?.serverId, screen.item.id) { Detail(screen.item, model, screen.source, screen.sources) }
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

@Composable
private fun Detail(item: ItemDetail, model: CatalogViewModel, source: ServerRef?, sources: List<ServerRef>) {
    LazyColumn(verticalArrangement = Arrangement.spacedBy(16.dp)) {
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(24.dp)) {
                Poster(item.posterUrl, model.api, Modifier.size(140.dp, 210.dp), source)
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(item.title, style = androidx.tv.material3.MaterialTheme.typography.headlineMedium)
                    item.year?.let { Text(it.toString()) }
                    if (source?.serverName?.isNotBlank() == true) Text(source.serverName)
                    if (sources.size > 1) Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        sources.forEach { available -> Button(onClick = { model.selectSource(available) }) { Text(available.serverName) } }
                    }
                    if (item.watched) Text(stringResource(R.string.watched))
                    item.overview?.let { Text(it, maxLines = 5, overflow = TextOverflow.Ellipsis) }
                    if (item.playableVersion != null || item.playTarget != null) {
                        val position = item.playTarget?.positionSeconds ?: item.positionSeconds
                        Button(onClick = { model.play(item) }, modifier = Modifier.focusInitially()) {
                            Text(if (position > 0) stringResource(R.string.resume, clock(position)) else stringResource(R.string.play))
                        }
                        if (position > 0 && item.playableVersion != null) {
                            Action(R.string.restart) { model.play(item, restart = true) }
                        }
                    } else if (item.children.isEmpty()) Text(stringResource(R.string.unavailable))
                }
            }
        }
        items(item.children, key = { it.id }) { child ->
            Button(
                onClick = { model.openItem(child.id, source) },
                modifier = Modifier.fillMaxWidth().then(
                    if (item.playableVersion == null && item.playTarget == null && child == item.children.first())
                        Modifier.focusInitially() else Modifier,
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
private fun MediaCard(item: ItemCard, api: PlysApi, modifier: Modifier, onClick: () -> Unit) {
    Button(onClick = onClick, shape = ButtonDefaults.shape(shape = RoundedCornerShape(8.dp)), modifier = modifier.fillMaxWidth()) {
        Column {
            Poster(item.posterUrl, api, Modifier.fillMaxWidth().height(144.dp))
            Spacer(Modifier.height(8.dp))
            Text(item.title, maxLines = 2, overflow = TextOverflow.Ellipsis)
            item.year?.let { Text(it.toString()) }
            if (item.watched) Text(stringResource(R.string.watched))
        }
    }
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
    Button(onClick = onClick, enabled = enabled, colors = ButtonDefaults.colors(
        disabledContentColor = androidx.tv.material3.MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.75f),
    )) { Text(stringResource(label)) }
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
