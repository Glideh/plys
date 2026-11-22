package fr.plys.tv.ui

import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.ui.Alignment
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.paging.LoadState
import androidx.paging.compose.collectAsLazyPagingItems
import androidx.paging.compose.itemKey
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.ClientException
import fr.plys.tv.data.Library

@Composable
fun CatalogScreen(screen: Screen.Catalog, model: CatalogViewModel, libraries: List<Library>) {
    // Each listing has its own pages and scroll: back to a catalog shows its titles where they were left, a new filter starts at the top.
    val items = screen.list.pages.collectAsLazyPagingItems()
    val grid = remember(screen.list) { LazyGridState(screen.focusedIndex) }
    var restored by remember(screen.list) { mutableStateOf(false) }
    LaunchedEffect(screen.list, items.itemCount) {
        if (!restored && items.itemCount > screen.focusedIndex) {
            grid.scrollToItem(screen.focusedIndex)
            restored = true
        }
    }
    var searchFocused by remember { mutableStateOf(false) }
    // A list replaced while typing in the search field leaves the focus there: its first card taking it would end the editing and close the keyboard.
    val cardsTakeFocus = remember(screen.list) { !searchFocused }
    Column(Modifier.fillMaxSize()) {
        Row(Modifier.padding(horizontal = FocusGutter), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Text(stringResource(kindLabel(screen.query.kind)), style = MaterialTheme.typography.headlineLarge, color = Plys.TextStrong)
            val total = screen.page.total
            val position = scrollPosition(screen.focusedPosition, total)
            Text(if (position != null) pluralStringResource(R.plurals.title_position, total, position, total)
                else pluralStringResource(R.plurals.title_count, total, total), style = MaterialTheme.typography.bodyLarge,
                color = Plys.Muted, modifier = Modifier.padding(bottom = 4.dp))
            if (screen.page.unavailableServers > 0) Text(partialResults(screen.page.unavailableServers, screen.page.unavailableServerNames),
                style = MaterialTheme.typography.bodySmall, color = Plys.Warning, modifier = Modifier.padding(bottom = 6.dp))
        }
        CatalogToolbar(screen, model, libraries) { searchFocused = it }
        if (items.itemCount == 0 && items.loadState.refresh is LoadState.NotLoading) EmptyState(stringResource(R.string.empty), Modifier.padding(horizontal = FocusGutter, vertical = 12.dp))
        val focusId = initialCardFocus(screen.focusedId, screen.page.items.firstOrNull()?.id, cardsTakeFocus)
        Box(Modifier.weight(1f).fillMaxWidth()) {
            LazyVerticalGrid(
                columns = GridCells.Adaptive(128.dp), state = grid, modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(start = FocusGutter, end = FocusGutter, top = FocusGutter, bottom = FocusGutter),
                horizontalArrangement = Arrangement.spacedBy(24.dp), verticalArrangement = Arrangement.spacedBy(24.dp),
            ) {
            items(count = items.itemCount, key = items.itemKey { it.id }) { index ->
                val card = items[index]
                if (card != null) MediaCard(card, model.api,
                    Modifier.focusInitially(card.id == focusId).onFocusChanged { if (it.hasFocus) model.rememberFocus(card.id, index) },
                    onPlay = { model.playCard(card) },
                ) { model.openCard(card) }
            }
            item(span = { GridItemSpan(maxLineSpan) }) {
                val loading = items.loadState.append is LoadState.Loading || items.loadState.refresh is LoadState.Loading
                val error = (items.loadState.append as? LoadState.Error)?.error ?: (items.loadState.refresh as? LoadState.Error)?.error
                if (loading) Box(Modifier.fillMaxWidth().padding(12.dp), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(Modifier.size(32.dp), color = Plys.Accent400, strokeWidth = 3.dp)
                }
                    if (error != null) ErrorBanner(if (error is ClientException) LocalContext.current.errorMessage(error) else stringResource(R.string.error_network),
                        { items.retry() })
                }
            }
            androidx.compose.animation.AnimatedVisibility(
                visible = grid.firstVisibleItemIndex > 0 || grid.firstVisibleItemScrollOffset > 0,
                enter = fadeIn(), exit = fadeOut(),
                modifier = Modifier.align(Alignment.TopCenter),
            ) {
                Box(Modifier.fillMaxWidth().height(16.dp).background(
                    Brush.verticalGradient(listOf(Plys.Surface950, Color.Transparent)),
                ))
            }
        }
    }
}

/** 1-based position of the focused title shown beside the total ("42/9394 titles"), or null when nothing is focused yet. */
internal fun scrollPosition(focusedPosition: Int?, total: Int): Int? =
    focusedPosition?.takeIf { total > 0 && it >= 0 }?.let { (it + 1).coerceAtMost(total) }

/**
 * The card that takes the focus when a list is shown: the one left focused, else the first. None when the list was
 * replaced while the search field held the focus, so that typing goes on with the keyboard open.
 */
internal fun initialCardFocus(focusedId: String?, firstId: String?, cardsTakeFocus: Boolean): String? =
    if (cardsTakeFocus) focusedId ?: firstId else null
