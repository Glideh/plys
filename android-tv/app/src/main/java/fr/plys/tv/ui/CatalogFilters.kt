package fr.plys.tv.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Checkbox
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.Alignment
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import fr.plys.tv.ui.PlysButton as Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.CatalogFilters
import fr.plys.tv.data.CatalogKind
import fr.plys.tv.data.CatalogQuery
import fr.plys.tv.data.ClientException
import fr.plys.tv.data.FilterOption
import fr.plys.tv.data.Library
import fr.plys.tv.data.PlysApi
import kotlinx.coroutines.delay

private val categories = listOf("genres", "excludeGenres", "actors", "creators", "keywords", "languages", "countries")
private val numbers = listOf("yearMin", "yearMax", "ratingMin", "votesMin", "runtimeMin", "runtimeMax")

fun filterSelection(filters: CatalogFilters, key: String): List<String> = when (key) {
    "genres" -> filters.genres.map(Int::toString)
    "excludeGenres" -> filters.excludeGenres.map(Int::toString)
    "actors" -> filters.actors.map(Int::toString)
    "creators" -> filters.creators.map(Int::toString)
    "keywords" -> filters.keywords.map(Int::toString)
    "languages" -> filters.languages
    "countries" -> filters.countries
    else -> emptyList()
}

fun withFilterSelection(filters: CatalogFilters, key: String, values: List<String>): CatalogFilters = when (key) {
    "genres" -> filters.copy(genres = values.mapNotNull(String::toIntOrNull))
    "excludeGenres" -> filters.copy(excludeGenres = values.mapNotNull(String::toIntOrNull))
    "actors" -> filters.copy(actors = values.mapNotNull(String::toIntOrNull))
    "creators" -> filters.copy(creators = values.mapNotNull(String::toIntOrNull))
    "keywords" -> filters.copy(keywords = values.mapNotNull(String::toIntOrNull))
    "languages" -> filters.copy(languages = values)
    "countries" -> filters.copy(countries = values)
    else -> filters
}

@Composable
private fun filterLabel(key: String, kind: CatalogKind): String = stringResource(when (key) {
    "genres" -> R.string.filter_genres
    "excludeGenres" -> R.string.filter_excluded_genres
    "actors" -> R.string.filter_actors
    "creators" -> if (kind == CatalogKind.MOVIES) R.string.filter_directors else R.string.filter_creators
    "keywords" -> R.string.filter_keywords
    "languages" -> R.string.filter_languages
    "countries" -> if (kind == CatalogKind.MOVIES) R.string.filter_countries else R.string.filter_origins
    "yearMin" -> R.string.filter_year_min
    "yearMax" -> R.string.filter_year_max
    "ratingMin" -> R.string.filter_rating
    "votesMin" -> R.string.filter_votes
    "runtimeMin" -> R.string.filter_runtime_min
    else -> R.string.filter_runtime_max
})

@Composable
fun CatalogToolbar(screen: Screen.Catalog, model: CatalogViewModel, libraries: List<Library>) {
    var search by remember(screen.query.kind, screen.query.libraryId) { mutableStateOf(screen.query.query) }
    var dialog by remember { mutableStateOf("") }
    LaunchedEffect(search) {
        if (search != screen.query.query) {
            delay(350)
            model.changeQuery(screen.query.copy(query = search, offset = 0))
        }
    }
    Row(Modifier.fillMaxWidth().padding(vertical = 4.dp), horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
        OutlinedTextField(search, { search = it }, Modifier.weight(1f), singleLine = true,
            label = { androidx.compose.material3.Text(stringResource(R.string.search)) })
        Button(onClick = { dialog = "library" }) {
            Text(libraries.firstOrNull { it.id == screen.query.libraryId }?.name ?: stringResource(R.string.all_libraries))
        }
        Button(onClick = { dialog = "sort" }) { Text(stringResource(R.string.sort)) }
        if (screen.query.kind != CatalogKind.OTHER) Button(onClick = { dialog = "filters" }) {
            Text(stringResource(R.string.filters_count, screen.query.filters.count))
        }
    }
    if (screen.query.filters.count > 0) {
        val filters = screen.query.filters
        val selected = categories.flatMap { key -> filterSelection(filters, key).map { key to it } }
        val bounds = numericFilterValues(filters).filterValues { it.isNotBlank() }
        LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.padding(vertical = 4.dp)) {
            items(selected, key = { "${it.first}:${it.second}" }) { (key, value) ->
                val label = "${filterLabel(key, screen.query.kind)}: ${model.api.filterOptionLabel(if (key == "excludeGenres") "genres" else key, value)}"
                Button(onClick = { model.changeQuery(screen.query.copy(filters = withFilterSelection(filters, key, filterSelection(filters, key) - value), offset = 0)) }) {
                    Text(stringResource(R.string.remove_filter, label))
                }
            }
            items(bounds.toList(), key = { it.first }) { (key, value) ->
                Button(onClick = { model.changeQuery(screen.query.copy(filters = withoutBound(filters, key), offset = 0)) }) {
                    Text(stringResource(R.string.remove_filter, "${filterLabel(key, screen.query.kind)}: $value"))
                }
            }
            item { Action(R.string.reset) { model.changeQuery(screen.query.copy(filters = CatalogFilters(), offset = 0)) } }
        }
    }
    when (dialog) {
        "library" -> ChoiceDialog(stringResource(R.string.libraries), { dialog = "" }) {
            item { Button(onClick = { model.changeQuery(screen.query.copy(libraryId = null, offset = 0)); dialog = "" }) { Text(stringResource(R.string.all_libraries)) } }
            items(libraries.filter { it.kind == screen.query.kind.value }, key = { it.id }) { library ->
                Button(onClick = { model.changeQuery(screen.query.copy(libraryId = library.id, offset = 0)); dialog = "" }) { Text(library.name) }
            }
        }
        "sort" -> ChoiceDialog(stringResource(R.string.sort), { dialog = "" }) {
            items(listOf("title" to R.string.sort_title, "year" to R.string.sort_year, "added" to R.string.sort_added, "rating" to R.string.sort_rating)) { (value, label) ->
                Button(onClick = { model.changeQuery(screen.query.copy(sort = value, offset = 0)); dialog = "" }) { Text(stringResource(label)) }
            }
        }
        "filters" -> FiltersDialog(screen.query, model.api, { dialog = "" }) {
            model.changeQuery(screen.query.copy(filters = it, offset = 0)); dialog = ""
        }
    }
}

@Composable
private fun ChoiceDialog(title: String, dismiss: () -> Unit, entries: androidx.compose.foundation.lazy.LazyListScope.() -> Unit) {
    Dialog(onDismissRequest = dismiss) {
        Surface(color = MaterialTheme.colorScheme.surface) {
            Column(Modifier.widthIn(max = 700.dp).fillMaxWidth().heightIn(max = 460.dp).padding(24.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(title, style = MaterialTheme.typography.titleLarge)
                LazyColumn(Modifier.weight(1f, fill = false), verticalArrangement = Arrangement.spacedBy(12.dp), content = entries)
                Action(R.string.back, onClick = dismiss)
            }
        }
    }
}

@Composable
private fun FiltersDialog(query: CatalogQuery, api: PlysApi, dismiss: () -> Unit, apply: (CatalogFilters) -> Unit) {
    var draft by remember { mutableStateOf(query.filters) }
    var numeric by remember { mutableStateOf(numericFilterValues(query.filters)) }
    var picker by remember { mutableStateOf<String?>(null) }
    var invalid by remember { mutableStateOf(false) }
    Dialog(onDismissRequest = dismiss) {
        Surface(color = MaterialTheme.colorScheme.surface) {
            Column(Modifier.widthIn(max = 760.dp).fillMaxWidth().heightIn(max = 480.dp).padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(stringResource(R.string.advanced_filters), style = MaterialTheme.typography.titleLarge)
                LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(categories) { category ->
                        Button(onClick = { picker = category }, modifier = Modifier.fillMaxWidth()) {
                            Text("${filterLabel(category, query.kind)} (${filterSelection(draft, category).size})")
                        }
                    }
                    item {
                        Button(onClick = { draft = draft.copy(genreMode = if (draft.genreMode == "all") "any" else "all") }) {
                            Text(stringResource(if (draft.genreMode == "all") R.string.genres_all else R.string.genres_any))
                        }
                    }
                    items(numbers.filter { query.kind == CatalogKind.MOVIES || !it.startsWith("runtime") }) { key ->
                        OutlinedTextField(numeric[key].orEmpty(), { numeric = numeric + (key to it); invalid = false }, Modifier.fillMaxWidth(),
                            label = { androidx.compose.material3.Text(filterLabel(key, query.kind)) }, singleLine = true,
                            keyboardOptions = KeyboardOptions(keyboardType = if (key == "ratingMin") KeyboardType.Decimal else KeyboardType.Number))
                    }
                }
                if (invalid) Text(stringResource(R.string.invalid_filters), color = MaterialTheme.colorScheme.error)
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Action(R.string.reset) { draft = CatalogFilters(); numeric = numericFilterValues(draft); invalid = false }
                    Action(R.string.cancel, onClick = dismiss)
                    Action(R.string.apply) {
                        val valuesValid = numeric.all { (key, value) -> value.isBlank() || if (key == "ratingMin") value.toDoubleOrNull() != null else value.toIntOrNull() != null }
                        val result = draft.copy(yearMin = numeric["yearMin"]?.toIntOrNull(), yearMax = numeric["yearMax"]?.toIntOrNull(),
                            ratingMin = numeric["ratingMin"]?.toDoubleOrNull(), votesMin = numeric["votesMin"]?.toIntOrNull(),
                            runtimeMin = if (query.kind == CatalogKind.MOVIES) numeric["runtimeMin"]?.toIntOrNull() else null,
                            runtimeMax = if (query.kind == CatalogKind.MOVIES) numeric["runtimeMax"]?.toIntOrNull() else null)
                        if (valuesValid && result.valid) apply(result) else invalid = true
                    }
                }
            }
        }
    }
    picker?.let { key ->
        FilterPickerDialog(query, key, draft, api, { picker = null }) { draft = withFilterSelection(draft, key, it); picker = null }
    }
}

private fun numericFilterValues(filters: CatalogFilters): Map<String, String> = mapOf(
    "yearMin" to filters.yearMin, "yearMax" to filters.yearMax, "ratingMin" to filters.ratingMin,
    "votesMin" to filters.votesMin, "runtimeMin" to filters.runtimeMin, "runtimeMax" to filters.runtimeMax,
).mapValues { it.value?.toString() ?: "" }

private fun withoutBound(filters: CatalogFilters, key: String): CatalogFilters = when (key) {
    "yearMin" -> filters.copy(yearMin = null)
    "yearMax" -> filters.copy(yearMax = null)
    "ratingMin" -> filters.copy(ratingMin = null)
    "votesMin" -> filters.copy(votesMin = null)
    "runtimeMin" -> filters.copy(runtimeMin = null)
    else -> filters.copy(runtimeMax = null)
}

@Composable
private fun FilterPickerDialog(query: CatalogQuery, category: String, draft: CatalogFilters, api: PlysApi, dismiss: () -> Unit, apply: (List<String>) -> Unit) {
    var search by remember { mutableStateOf("") }
    var selected by remember { mutableStateOf(filterSelection(draft, category)) }
    var options by remember { mutableStateOf(emptyList<FilterOption>()) }
    var error by remember { mutableStateOf("") }
    var loading by remember { mutableStateOf(false) }
    val context = LocalContext.current
    val locale = if (LocalConfiguration.current.locales[0].language == "fr") "fr" else "en"
    val excluded = when (category) { "genres" -> filterSelection(draft, "excludeGenres"); "excludeGenres" -> filterSelection(draft, "genres"); else -> emptyList() }
    LaunchedEffect(search, locale) {
        delay(300)
        loading = true
        error = ""
        try { options = api.filterOptions(query.kind, if (category == "excludeGenres") "genres" else category, search, query.libraryId, selected, locale).options[if (category == "excludeGenres") "genres" else category].orEmpty() }
        catch (failure: ClientException) { error = context.errorMessage(failure) }
        finally { loading = false }
    }
    Dialog(onDismissRequest = dismiss) {
        Surface(color = MaterialTheme.colorScheme.surface) {
            Column(Modifier.widthIn(max = 700.dp).fillMaxWidth().heightIn(max = 480.dp).padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(filterLabel(category, query.kind), style = MaterialTheme.typography.titleLarge)
                OutlinedTextField(search, { search = it }, Modifier.fillMaxWidth().focusInitially(), singleLine = true,
                    label = { androidx.compose.material3.Text(stringResource(R.string.search)) })
                if (loading) Text(stringResource(R.string.loading))
                if (error.isNotBlank()) Text(error, color = MaterialTheme.colorScheme.error)
                LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (!loading && options.isEmpty()) item { Text(stringResource(R.string.empty)) }
                    items(options, key = { it.id }) { option ->
                        Button(onClick = { selected = if (option.id in selected) selected - option.id else selected + option.id },
                            enabled = option.id !in excluded && (option.id in selected || selected.size < 20), modifier = Modifier.fillMaxWidth()) {
                            Checkbox(checked = option.id in selected, onCheckedChange = null)
                            Text(option.label, modifier = Modifier.weight(1f))
                        }
                    }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Action(R.string.reset) { selected = emptyList() }
                    Action(R.string.cancel, onClick = dismiss)
                    Action(R.string.apply) { apply(selected) }
                }
            }
        }
    }
}