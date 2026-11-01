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
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Button
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import fr.plys.tv.R
import fr.plys.tv.data.ItemCard
import fr.plys.tv.data.ChildItem
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.PlysApi
import kotlin.math.max

@Composable
fun CatalogApp(model: CatalogViewModel, state: CatalogState, modifier: Modifier = Modifier) {
    BackHandler(model.canGoBack || state.loading) { model.back() }
    Column(modifier.padding(horizontal = 48.dp, vertical = 24.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Text("Plys TV", modifier = Modifier.weight(1f))
            if (model.canGoBack) Action(R.string.back) { model.back() }
            if (state.screen != Screen.Login) Action(R.string.sign_out, !state.loading) { model.logout() }
        }
        Spacer(Modifier.height(16.dp))
        state.error?.let {
            Text(LocalContext.current.errorMessage(it), color = Color(0xFFFCA5A5))
            if (state.canRetry) Action(R.string.retry, !state.loading) { model.retry() }
            Spacer(Modifier.height(12.dp))
        }
        if (state.loading) {
            CircularProgressIndicator(color = Color(0xFFA5B4FC))
            Text(stringResource(R.string.loading))
        }
        if (state.screen == Screen.Login) {
            Login(model, state.loading)
        } else if (!state.loading) {
            when (val screen = state.screen) {
                Screen.Login -> Unit
                is Screen.Libraries -> {
                    Text(stringResource(R.string.libraries))
                    if (screen.items.isEmpty()) Text(stringResource(R.string.empty))
                    key("libraries") {
                        val focus = initialFocus(screen.items.isNotEmpty())
                        LazyColumn(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            items(screen.items, key = { it.id }) { library ->
                                Button(
                                    onClick = { model.openLibrary(library) },
                                    modifier = Modifier.fillMaxWidth().then(
                                        if (library == screen.items.first()) Modifier.focusRequester(focus) else Modifier,
                                    ),
                                ) { Text(library.name) }
                            }
                        }
                    }
                }
                is Screen.Catalog -> key(screen.library.id, screen.page.offset) {
                    Text(screen.library.name)
                    val page = screen.page
                    Row(horizontalArrangement = Arrangement.spacedBy(16.dp), modifier = Modifier.padding(vertical = 12.dp)) {
                        Action(R.string.previous_page, page.offset > 0) { model.page(max(0, page.offset - page.limit)) }
                        Text(stringResource(R.string.page, page.offset / page.limit + 1, max(1, (page.total + page.limit - 1) / page.limit)))
                        Action(R.string.next_page, page.offset + page.limit < page.total) { model.page(page.offset + page.limit) }
                    }
                    if (page.items.isEmpty()) Text(stringResource(R.string.empty))
                    val focus = initialFocus(page.items.isNotEmpty())
                    LazyVerticalGrid(
                        columns = GridCells.Adaptive(160.dp),
                        horizontalArrangement = Arrangement.spacedBy(16.dp),
                        verticalArrangement = Arrangement.spacedBy(16.dp),
                    ) {
                        items(page.items, key = { it.id }) { item ->
                            MediaCard(item, model.api,
                                if (item == page.items.first()) Modifier.focusRequester(focus) else Modifier,
                            ) { model.openItem(item.id) }
                        }
                    }
                }
                is Screen.Detail -> key(screen.item.id) { Detail(screen.item, model) }
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
    val focus = initialFocus(!loading)
    Column(
        Modifier.width(520.dp).verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        OutlinedTextField(server, { server = it }, Modifier.fillMaxWidth().focusRequester(focus),
            label = { Text(stringResource(R.string.server)) }, singleLine = true, enabled = !loading,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri))
        OutlinedTextField(username, { username = it }, Modifier.fillMaxWidth(),
            label = { Text(stringResource(R.string.username)) }, singleLine = true, enabled = !loading)
        OutlinedTextField(password, { password = it }, Modifier.fillMaxWidth(),
            label = { Text(stringResource(R.string.password)) }, singleLine = true, enabled = !loading,
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
private fun Detail(item: ItemDetail, model: CatalogViewModel) {
    val focus = initialFocus(item.playableVersion != null || item.playTarget != null || item.children.isNotEmpty())
    LazyColumn(verticalArrangement = Arrangement.spacedBy(16.dp)) {
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(24.dp)) {
                Poster(item.posterUrl, model.api, Modifier.size(140.dp, 210.dp))
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(item.title, style = androidx.tv.material3.MaterialTheme.typography.headlineMedium)
                    item.year?.let { Text(it.toString()) }
                    if (item.watched) Text(stringResource(R.string.watched))
                    item.overview?.let { Text(it, maxLines = 5, overflow = TextOverflow.Ellipsis) }
                    if (item.playableVersion != null || item.playTarget != null) {
                        val position = item.playTarget?.positionSeconds ?: item.positionSeconds
                        Button(onClick = { model.play(item) }, modifier = Modifier.focusRequester(focus)) {
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
                onClick = { model.openItem(child.id) },
                modifier = Modifier.fillMaxWidth().then(
                    if (item.playableVersion == null && item.playTarget == null && child == item.children.first())
                        Modifier.focusRequester(focus) else Modifier,
                ),
            ) {
                Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                    Poster(child.thumbUrl ?: child.posterUrl, model.api, Modifier.size(96.dp, 54.dp))
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
    Button(onClick = onClick, modifier = modifier.fillMaxWidth()) {
        Column {
            Poster(item.posterUrl, api, Modifier.fillMaxWidth().height(210.dp))
            Spacer(Modifier.height(8.dp))
            Text(item.title, maxLines = 2, overflow = TextOverflow.Ellipsis)
            item.year?.let { Text(it.toString()) }
            if (item.watched) Text(stringResource(R.string.watched))
        }
    }
}

@Composable
private fun Poster(path: String?, api: PlysApi, modifier: Modifier) {
    Box(modifier.background(Color(0xFF374151))) {
        if (path != null) AsyncImage(
            model = api.mediaUrl(path), contentDescription = null,
            modifier = Modifier.fillMaxSize(), contentScale = ContentScale.Crop,
        )
    }
}

@Composable
fun Action(label: Int, enabled: Boolean = true, onClick: () -> Unit) {
    Button(onClick = onClick, enabled = enabled) { Text(stringResource(label)) }
}

@Composable
private fun initialFocus(enabled: Boolean = true): FocusRequester {
    val requester = remember { FocusRequester() }
    LaunchedEffect(enabled) {
        if (enabled) {
            androidx.compose.runtime.withFrameNanos { }
            requester.requestFocus()
        }
    }
    return requester
}
