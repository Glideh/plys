package fr.plys.tv.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R

/** The titles the user plans to watch, from every server, most recently added first. */
@Composable
fun WatchlistScreen(screen: Screen.Watchlist, model: CatalogViewModel) {
    val items = screen.data.items
    Column(Modifier.fillMaxSize()) {
        Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Text(stringResource(R.string.watchlist), style = MaterialTheme.typography.headlineLarge, color = Plys.TextStrong)
            Text(pluralStringResource(R.plurals.title_count, items.size, items.size), style = MaterialTheme.typography.bodyLarge,
                color = Plys.Muted, modifier = Modifier.padding(bottom = 4.dp))
            if (screen.data.unavailableServers > 0) Text(stringResource(R.string.partial_results, screen.data.unavailableServers),
                style = MaterialTheme.typography.bodySmall, color = Plys.Warning, modifier = Modifier.padding(bottom = 6.dp))
        }
        if (items.isEmpty()) EmptyState(stringResource(R.string.watchlist_empty), Modifier.padding(top = 12.dp))
        LazyVerticalGrid(
            columns = GridCells.Adaptive(128.dp), modifier = Modifier.weight(1f).fillMaxWidth().padding(top = 12.dp),
            contentPadding = PaddingValues(FocusGutter), horizontalArrangement = Arrangement.spacedBy(24.dp), verticalArrangement = Arrangement.spacedBy(24.dp),
        ) {
            itemsIndexed(items, key = { _, item -> "${item.source.serverId}/${item.id}" }) { index, item ->
                PosterTile(title = item.title, image = item.posterUrl, api = model.api, source = item.source,
                    modifier = Modifier.focusInitially(index == 0), landscape = item.kind == "video",
                    subtitle = listOfNotNull(item.year?.toString(), item.source.serverName.takeIf { item.source.serverUrl != null && it.isNotBlank() })
                        .joinToString(" · ")) { model.openWatchlistItem(item) }
            }
        }
    }
}
