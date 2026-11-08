package fr.plys.tv.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.Logout
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Done
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.FavoriteBorder
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.Lock
import androidx.compose.material.icons.outlined.Pause
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.RemoveDone
import androidx.compose.material.icons.outlined.Replay
import androidx.compose.material.icons.outlined.SkipNext
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import fr.plys.tv.R
import fr.plys.tv.data.ItemCard
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.ServerRef
import fr.plys.tv.ui.PlysButton as Button

@Composable
fun CatalogApp(model: CatalogViewModel, state: CatalogState, modifier: Modifier = Modifier) {
    BackHandler(model.canGoBack || state.loading) { model.back() }
    if (state.screen == Screen.Login) {
        Box(modifier) {
            Login(model, state)
            if (state.loading) BusyIndicator(Modifier.align(Alignment.TopEnd))
        }
        return
    }
    val section = when (val screen = state.screen) {
        is Screen.Home -> Section.HOME
        is Screen.Watchlist -> Section.WATCHLIST
        is Screen.Catalog -> sectionOf(screen.query.kind)
        else -> Section.NONE
    }
    Box(modifier) {
        // The detail page draws its backdrop under the rail; the other screens start after it.
        val detail = state.screen is Screen.Detail
        Column(Modifier.fillMaxSize().padding(start = if (detail) 0.dp else RailWidth + 24.dp, end = if (detail) 0.dp else 40.dp, top = if (detail) 0.dp else 20.dp)) {
            state.error?.let {
                ErrorBanner(LocalContext.current.errorMessage(it), if (state.canRetry) ({ model.retry() }) else null,
                    Modifier.padding(start = if (detail) RailWidth + 24.dp else 0.dp, end = if (detail) 40.dp else 0.dp, top = if (detail) 20.dp else 0.dp, bottom = 12.dp))
            }
            when (val screen = state.screen) {
                Screen.Login -> Unit
                is Screen.Home -> key("home") { HomeScreen(screen.data, model, state.libraries, state.catalogCounts, state.recent) }
                is Screen.Libraries -> key("libraries") {
                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        Text(stringResource(R.string.libraries), style = MaterialTheme.typography.headlineMedium)
                        if (screen.items.isEmpty()) EmptyState(stringResource(R.string.empty))
                        LazyColumn(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            items(screen.items, key = { it.id }) { library ->
                                Button(onClick = { model.openLibrary(library) },
                                    modifier = Modifier.fillMaxWidth().focusInitially(library == screen.items.first())) { Text(library.name) }
                            }
                        }
                    }
                }
                is Screen.Watchlist -> key("watchlist") { WatchlistScreen(screen, model) }
                is Screen.Catalog -> key(screen.query) { CatalogScreen(screen, model, state.libraries) }
                is Screen.Detail -> key(screen.source?.serverId, screen.item.id) { DetailScreen(screen, model) }
            }
        }
        SideRail(section, Modifier.align(Alignment.CenterStart), home = model::home, watchlist = model::openWatchlist, openKind = model::openKind, signOut = model::logout)
        if (state.loading) BusyIndicator(Modifier.align(Alignment.TopEnd))
    }
}

/** Loading never moves the screen: a small spinner in the corner, where TV apps show activity. */
@Composable
private fun BusyIndicator(modifier: Modifier) {
    Box(modifier.padding(24.dp).size(44.dp).background(Plys.Surface900.copy(alpha = 0.9f), Plys.Card), contentAlignment = Alignment.Center) {
        CircularProgressIndicator(Modifier.size(24.dp), color = Plys.Accent400, strokeWidth = 3.dp)
    }
}

@Composable
fun ErrorBanner(message: String, retry: (() -> Unit)?, modifier: Modifier = Modifier) {
    Row(
        modifier.fillMaxWidth().background(Plys.Error.copy(alpha = 0.1f), Plys.Card).border(1.dp, Plys.Error.copy(alpha = 0.3f), Plys.Card)
            .padding(horizontal = 20.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Icon(Icons.Outlined.ErrorOutline, contentDescription = null, tint = Plys.Error, modifier = Modifier.size(24.dp))
        Text(message, color = Plys.Error, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
        if (retry != null) Action(R.string.retry, onClick = retry)
    }
}

@Composable
fun EmptyState(message: String, modifier: Modifier = Modifier) {
    Text(message, style = MaterialTheme.typography.bodyLarge, color = Plys.Muted,
        modifier = modifier.fillMaxWidth().border(1.dp, Plys.Surface600, Plys.Card).padding(32.dp))
}

/** The web's sign-in card, centred on the brand: one column of fields, the remote moves down through them. */
@Composable
private fun Login(model: CatalogViewModel, state: CatalogState) {
    val loading = state.loading
    var server by rememberSaveable { mutableStateOf(model.api.savedServer) }
    var username by rememberSaveable { mutableStateOf("") }
    // Do not put credentials into saved instance state.
    var password by remember { mutableStateOf("") }
    val fieldColors = OutlinedTextFieldDefaults.colors(
        focusedBorderColor = Plys.Accent400, unfocusedBorderColor = Plys.Surface600, focusedLabelColor = Plys.Accent300,
        unfocusedLabelColor = Plys.Muted, cursorColor = Plys.Accent400,
        focusedContainerColor = Plys.Surface800, unfocusedContainerColor = Plys.Surface800.copy(alpha = 0.5f),
    )
    Box(Modifier.fillMaxSize().background(androidx.compose.ui.graphics.Brush.radialGradient(
        listOf(Plys.Accent600.copy(alpha = 0.18f), Plys.Surface950), radius = 1400f)), contentAlignment = Alignment.Center) {
        Column(
            Modifier.width(560.dp).background(Plys.Surface900.copy(alpha = 0.85f), Plys.Large).border(1.dp, Plys.Hairline, Plys.Large)
                .verticalScroll(rememberScrollState()).padding(horizontal = 40.dp, vertical = 32.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Brand()
            Text(stringResource(R.string.sign_in_title), style = MaterialTheme.typography.bodyLarge, color = Plys.Muted)
            state.error?.let { Text(LocalContext.current.errorMessage(it), color = Plys.Error, style = MaterialTheme.typography.bodyMedium) }
            OutlinedTextField(server, { server = it }, Modifier.fillMaxWidth().focusInitially(!loading),
                label = { androidx.compose.material3.Text(stringResource(R.string.server)) }, singleLine = true, enabled = !loading,
                colors = fieldColors, shape = Plys.Small, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri))
            OutlinedTextField(username, { username = it }, Modifier.fillMaxWidth(),
                label = { androidx.compose.material3.Text(stringResource(R.string.username)) }, singleLine = true, enabled = !loading,
                colors = fieldColors, shape = Plys.Small)
            OutlinedTextField(password, { password = it }, Modifier.fillMaxWidth(),
                label = { androidx.compose.material3.Text(stringResource(R.string.password)) }, singleLine = true, enabled = !loading,
                colors = fieldColors, shape = Plys.Small, visualTransformation = PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password))
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Icon(Icons.Outlined.Lock, contentDescription = null, tint = Plys.Subtle, modifier = Modifier.size(16.dp))
                Text(stringResource(R.string.http_warning), style = MaterialTheme.typography.bodySmall, color = Plys.Subtle)
            }
            Button(onClick = {
                model.login(server, username.trim(), password)
                password = ""
            }, primary = true, modifier = Modifier.fillMaxWidth(),
                enabled = !loading && server.isNotBlank() && username.isNotBlank() && password.isNotEmpty()) {
                Text(stringResource(R.string.sign_in), modifier = Modifier.fillMaxWidth(), textAlign = androidx.compose.ui.text.style.TextAlign.Center)
            }
        }
    }
}

@Composable
fun MediaCard(item: ItemCard, api: PlysApi, modifier: Modifier, onClick: () -> Unit) {
    PosterTile(title = item.title, image = item.posterUrl, api = api, modifier = modifier,
        subtitle = listOfNotNull(item.year?.toString(), item.rating?.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
            item.sources.firstOrNull()?.serverName?.takeIf { it.isNotBlank() }).joinToString(" · "),
        landscape = item.kind == "video", watched = item.watched, onClick = onClick)
}

@Composable
fun Poster(path: String?, api: PlysApi, modifier: Modifier, source: ServerRef? = null, contentScale: ContentScale = ContentScale.Crop) {
    Box(modifier.background(Plys.Surface800)) {
        if (path != null) AsyncImage(
            model = api.posterUrl(path, source), contentDescription = null,
            modifier = Modifier.fillMaxSize(), contentScale = contentScale,
        )
    }
}

@Composable
fun Action(label: Int, enabled: Boolean = true, modifier: Modifier = Modifier, primary: Boolean = false, onClick: () -> Unit) {
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
        // The heart is outlined while the title is not in the watchlist, filled once it is.
        R.string.add_to_watchlist -> Icons.Outlined.FavoriteBorder
        R.string.remove_from_watchlist -> Icons.Filled.Favorite
        R.string.next_episode, R.string.play_next_now -> Icons.Outlined.SkipNext
        else -> null
    }
    Button(onClick = onClick, enabled = enabled, modifier = modifier, primary = primary) {
        if (icon != null) {
            val tint = if (label == R.string.remove_from_watchlist) Plys.Favorite else LocalContentColor.current
            Icon(icon, contentDescription = null, modifier = Modifier.size(20.dp), tint = tint); Spacer(Modifier.width(8.dp))
        }
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
