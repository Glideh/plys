package fr.plys.tv.ui

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
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
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
    val items = model.catalogPages.collectAsLazyPagingItems()
    val grid = rememberLazyGridState(initialFirstVisibleItemIndex = screen.focusedIndex)
    var restored by remember { mutableStateOf(false) }
    LaunchedEffect(items.itemCount) {
        if (!restored && items.itemCount > screen.focusedIndex) {
            grid.scrollToItem(screen.focusedIndex)
            restored = true
        }
    }
    Column(Modifier.fillMaxSize()) {
        Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Text(stringResource(kindLabel(screen.query.kind)), style = MaterialTheme.typography.headlineLarge, color = Plys.TextStrong)
            Text(pluralStringResource(R.plurals.title_count, screen.page.total, screen.page.total), style = MaterialTheme.typography.bodyLarge,
                color = Plys.Muted, modifier = Modifier.padding(bottom = 4.dp))
            if (screen.page.unavailableServers > 0) Text(stringResource(R.string.partial_results, screen.page.unavailableServers),
                style = MaterialTheme.typography.bodySmall, color = Plys.Warning, modifier = Modifier.padding(bottom = 6.dp))
        }
        CatalogToolbar(screen, model, libraries)
        if (items.itemCount == 0 && items.loadState.refresh is LoadState.NotLoading) EmptyState(stringResource(R.string.empty), Modifier.padding(top = 12.dp))
        val focusId = screen.focusedId ?: screen.page.items.firstOrNull()?.id
        LazyVerticalGrid(
            columns = GridCells.Adaptive(128.dp), state = grid, modifier = Modifier.weight(1f).fillMaxWidth(),
            contentPadding = PaddingValues(FocusGutter), horizontalArrangement = Arrangement.spacedBy(24.dp), verticalArrangement = Arrangement.spacedBy(24.dp),
        ) {
            items(count = items.itemCount, key = items.itemKey { it.id }) { index ->
                val card = items[index]
                if (card != null) MediaCard(card, model.api,
                    Modifier.focusInitially(card.id == focusId).onFocusChanged { if (it.hasFocus) model.rememberFocus(card.id, index) },
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
    }
}