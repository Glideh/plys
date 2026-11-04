package fr.plys.tv.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.CatalogKind
import fr.plys.tv.data.HomeItem
import fr.plys.tv.data.HomeResponse

fun kindLabel(kind: CatalogKind): Int = when (kind) {
    CatalogKind.MOVIES -> R.string.movies
    CatalogKind.SERIES -> R.string.series
    CatalogKind.OTHER -> R.string.other
}

@Composable
fun HomeScreen(data: HomeResponse, model: CatalogViewModel) {
    val firstRail = data.continueWatching.ifEmpty { data.nextUp }
    LazyColumn(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        if (data.unavailableServers > 0) item { Text(stringResource(R.string.partial_results, data.unavailableServers), color = MaterialTheme.colorScheme.onSurfaceVariant) }
        if (data.continueWatching.isNotEmpty()) item {
            HomeRail(R.string.continue_watching, data.continueWatching, model, true)
        }
        if (data.nextUp.isNotEmpty()) item {
            HomeRail(R.string.next_up, data.nextUp, model, data.continueWatching.isEmpty())
        }
        item {
            Row(Modifier.fillMaxWidth().padding(8.dp), horizontalArrangement = Arrangement.spacedBy(24.dp)) {
                CatalogKind.entries.forEach { kind ->
                    Button(onClick = { model.openKind(kind) }, shape = ButtonDefaults.shape(shape = RoundedCornerShape(8.dp)), modifier = Modifier.weight(1f).height(88.dp).focusInitially(firstRail.isEmpty() && kind == CatalogKind.MOVIES)) {
                        Text(stringResource(kindLabel(kind)), style = MaterialTheme.typography.headlineSmall)
                    }
                }
            }
        }
    }
}

@Composable
private fun HomeRail(title: Int, entries: List<HomeItem>, model: CatalogViewModel, requestInitial: Boolean) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(stringResource(title), style = MaterialTheme.typography.titleLarge)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(16.dp), modifier = Modifier.padding(vertical = 4.dp)) {
            items(entries, key = { it.source.serverId + it.id }) { item ->
                Button(onClick = { model.openItem(item.id, item.source) }, shape = ButtonDefaults.shape(shape = RoundedCornerShape(8.dp)), modifier = Modifier.width(280.dp).height(104.dp).focusInitially(requestInitial && item == entries.first())) {
                    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        Poster(item.posterUrl, model.api, Modifier.size(48.dp, 72.dp), item.source)
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(item.showTitle.ifBlank { item.title }, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            if (item.showTitle.isNotBlank()) Text(item.title, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            if (item.source.serverName.isNotBlank()) Text(item.source.serverName, style = MaterialTheme.typography.labelSmall)
                            if (item.positionSeconds > 0) {
                                Text(clock(item.positionSeconds), style = MaterialTheme.typography.labelMedium)
                                if (item.durationSeconds > 0) LinearProgressIndicator(progress = { (item.positionSeconds / item.durationSeconds).toFloat().coerceIn(0f, 1f) }, modifier = Modifier.fillMaxWidth())
                            }
                        }
                    }
                }
            }
        }
    }
}