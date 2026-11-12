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
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.FilterList
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material.icons.automirrored.outlined.Sort
import androidx.compose.material3.Icon
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.material3.RadioButton
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
import androidx.compose.foundation.clickable
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
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
private val choices = listOf("resolution", "audio")

/** Related bounds, shown side by side under one heading: title, then the label of each bound. */
private data class NumberGroup(val title: Int, val fields: List<Pair<String, Int>>)
private val numberGroups = listOf(
    NumberGroup(R.string.filter_group_year, listOf("yearMin" to R.string.filter_from, "yearMax" to R.string.filter_to)),
    NumberGroup(R.string.filter_group_rating, listOf("ratingMin" to R.string.filter_min_rating, "votesMin" to R.string.filter_min_votes)),
    NumberGroup(R.string.filter_group_runtime, listOf("runtimeMin" to R.string.filter_minimum, "runtimeMax" to R.string.filter_maximum)),
)

fun choiceValue(filters: CatalogFilters, key: String): String? = if (key == "resolution") filters.resolution else filters.audio

fun withChoice(filters: CatalogFilters, key: String, value: String?): CatalogFilters =
    if (key == "resolution") filters.copy(resolution = value) else filters.copy(audio = value)

@Composable
private fun choiceLabel(key: String, value: String): String = if (key == "resolution") when (value) {
    "2160p" -> stringResource(R.string.resolution_2160p)
    "1080p" -> stringResource(R.string.resolution_1080p)
    "720p" -> stringResource(R.string.resolution_720p)
    else -> value
} else when (value) {
    "atmos" -> "Dolby Atmos"
    "truehd" -> "Dolby TrueHD"
    "eac3" -> "Dolby Digital Plus"
    "ac3" -> "Dolby Digital"
    "dtsHdMa" -> "DTS-HD Master Audio"
    "dts" -> stringResource(R.string.audio_dts_all)
    "pcm" -> stringResource(R.string.audio_pcm)
    "opus" -> "Opus"
    else -> value.uppercase()
}

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
    "genres" -> filters.copy(genres = values.take(1).mapNotNull(String::toIntOrNull))
    "excludeGenres" -> filters.copy(excludeGenres = values.take(1).mapNotNull(String::toIntOrNull))
    "actors" -> filters.copy(actors = values.take(1).mapNotNull(String::toIntOrNull))
    "creators" -> filters.copy(creators = values.take(1).mapNotNull(String::toIntOrNull))
    "keywords" -> filters.copy(keywords = values.take(1).mapNotNull(String::toIntOrNull))
    "languages" -> filters.copy(languages = values.take(1))
    "countries" -> filters.copy(countries = values.take(1))
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
    "resolution" -> R.string.filter_resolution
    "audio" -> R.string.filter_audio
    else -> R.string.filter_runtime_max
})

/**
 * A one-line text field in the colours of the web inputs. Moving onto it only focuses it: the on-screen keyboard
 * would otherwise open on the way and block the navigation. Selecting it (OK) starts editing and opens the keyboard;
 * Done, or moving away, ends it.
 */
@Composable
fun SearchField(value: String, onValueChange: (String) -> Unit, label: String, modifier: Modifier = Modifier, height: Dp = 40.dp) {
    var focused by remember { mutableStateOf(false) }
    var editing by remember { mutableStateOf(false) }
    val box = remember { FocusRequester() }
    val field = remember { FocusRequester() }
    val keyboard = LocalSoftwareKeyboardController.current
    LaunchedEffect(editing) {
        if (editing) {
            field.requestFocus()
            keyboard?.show()
        }
    }
    val done = {
        keyboard?.hide()
        box.requestFocus()
        editing = false
    }
    Box(
        modifier.height(height)
            .background(if (focused) Plys.Surface700 else Plys.Surface800, Plys.Small)
            .border(if (focused) 2.dp else 1.dp, if (focused) Plys.Accent400 else Plys.Hairline, Plys.Small)
            .focusRequester(box)
            .onFocusChanged { focused = it.hasFocus }
            .clickable(interactionSource = null, indication = null) { editing = true }
            .padding(horizontal = 12.dp).semantics { contentDescription = label },
        contentAlignment = Alignment.CenterStart,
    ) {
        BasicTextField(value, onValueChange,
            modifier = Modifier.fillMaxSize().focusRequester(field)
                // Out of the focus path until selected, so the remote passes over it without opening the keyboard.
                .focusProperties { canFocus = editing }
                .onFocusChanged { if (!it.isFocused && editing) editing = false }
                // OK on the field again reopens a keyboard closed with Back.
                .onPreviewKeyEvent {
                    if (it.key == Key.DirectionCenter && it.type == KeyEventType.KeyUp) { keyboard?.show(); true } else false
                },
            singleLine = true, textStyle = MaterialTheme.typography.bodyMedium.copy(color = MaterialTheme.colorScheme.onSurface),
            cursorBrush = SolidColor(MaterialTheme.colorScheme.primary),
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done), keyboardActions = KeyboardActions(onDone = { done() }),
            decorationBox = { input ->
                Row(Modifier.fillMaxSize(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Icon(Icons.Outlined.Search, contentDescription = null, modifier = Modifier.size(18.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                    Box(Modifier.weight(1f)) {
                        if (value.isEmpty()) Text(label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        input()
                    }
                }
            },
        )
    }
}

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
        SearchField(search, { search = it }, stringResource(R.string.search), Modifier.weight(1f))
        if (libraries.count { it.kind == screen.query.kind.value } > 1) ToolbarButton(Icons.Outlined.VideoLibrary,
            libraries.firstOrNull { it.id == screen.query.libraryId }?.name ?: stringResource(R.string.all_libraries)) { dialog = "library" }
        ToolbarButton(Icons.AutoMirrored.Outlined.Sort, "${stringResource(R.string.sort)} : ${stringResource(sortLabel(screen.query.sort))}") { dialog = "sort" }
        if (screen.query.kind != CatalogKind.OTHER) ToolbarButton(Icons.Outlined.FilterList, stringResource(R.string.filters_count, screen.query.filters.count),
            highlighted = screen.query.filters.count > 0) { dialog = "filters" }
    }
    if (screen.query.filters.count > 0) {
        val filters = screen.query.filters
        val selected = categories.flatMap { key -> filterSelection(filters, key).map { key to it } }
        val bounds = numericFilterValues(filters).filterValues { it.isNotBlank() }
        val technical = choices.mapNotNull { key -> choiceValue(filters, key)?.let { key to it } }
        LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.padding(vertical = 4.dp)) {
            items(selected, key = { "${it.first}:${it.second}" }) { (key, value) ->
                val label = "${filterLabel(key, screen.query.kind)}: ${model.api.filterOptionLabel(if (key == "excludeGenres") "genres" else key, value)}"
                Button(onClick = { model.changeQuery(screen.query.copy(filters = withFilterSelection(filters, key, filterSelection(filters, key) - value), offset = 0)) }) {
                    Text(stringResource(R.string.remove_filter, label))
                }
            }
            items(technical, key = { it.first }) { (key, value) ->
                Button(onClick = { model.changeQuery(screen.query.copy(filters = withChoice(filters, key, null), offset = 0)) }) {
                    Text(stringResource(R.string.remove_filter, "${filterLabel(key, screen.query.kind)}: ${choiceLabel(key, value)}"))
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
            item { Choice(stringResource(R.string.all_libraries), screen.query.libraryId == null) { model.changeQuery(screen.query.copy(libraryId = null, offset = 0)); dialog = "" } }
            items(libraries.filter { it.kind == screen.query.kind.value }, key = { it.id }) { library ->
                Choice(library.name, library.id == screen.query.libraryId) { model.changeQuery(screen.query.copy(libraryId = library.id, offset = 0)); dialog = "" }
            }
        }
        "sort" -> ChoiceDialog(stringResource(R.string.sort), { dialog = "" }) {
            items(listOf("title", "year", "added", "rating")) { value ->
                Choice(stringResource(sortLabel(value)), value == screen.query.sort) { model.changeQuery(screen.query.copy(sort = value, offset = 0)); dialog = "" }
            }
        }
        "filters" -> FiltersDialog(screen.query, model.api, { dialog = "" }) {
            model.changeQuery(screen.query.copy(filters = it, offset = 0)); dialog = ""
        }
    }
}

private fun sortLabel(sort: String): Int = when (sort) {
    "year" -> R.string.sort_year
    "added" -> R.string.sort_added
    "rating" -> R.string.sort_rating
    else -> R.string.sort_title
}

@Composable
private fun ToolbarButton(icon: androidx.compose.ui.graphics.vector.ImageVector, label: String, highlighted: Boolean = false, onClick: () -> Unit) {
    Button(onClick = onClick, modifier = Modifier.height(40.dp), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 14.dp)) {
        val content = androidx.tv.material3.LocalContentColor.current
        Icon(icon, contentDescription = null, modifier = Modifier.size(18.dp),
            tint = if (highlighted && content == Plys.Text) Plys.Accent400 else content)
        androidx.compose.foundation.layout.Spacer(Modifier.size(8.dp))
        Text(label, style = MaterialTheme.typography.bodyMedium, maxLines = 1)
    }
}

@Composable
fun ChoiceDialog(title: String, dismiss: () -> Unit, entries: androidx.compose.foundation.lazy.LazyListScope.() -> Unit) {
    Dialog(onDismissRequest = dismiss) {
        Surface(color = Plys.Surface900, shape = Plys.Large, border = androidx.compose.foundation.BorderStroke(1.dp, Plys.Hairline)) {
            Column(Modifier.widthIn(max = 700.dp).fillMaxWidth().heightIn(max = 460.dp).padding(24.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(title, style = MaterialTheme.typography.headlineSmall, color = Plys.TextStrong)
                LazyColumn(Modifier.weight(1f, fill = false), verticalArrangement = Arrangement.spacedBy(10.dp), contentPadding = androidx.compose.foundation.layout.PaddingValues(8.dp), content = entries)
                Action(R.string.back, onClick = dismiss)
            }
        }
    }
}

@Composable
private fun FiltersDialog(query: CatalogQuery, api: PlysApi, dismiss: () -> Unit, apply: (CatalogFilters) -> Unit) {
    var draft by remember { mutableStateOf(categories.fold(query.filters) { filters, key -> withFilterSelection(filters, key, filterSelection(filters, key)) }) }
    var numeric by remember { mutableStateOf(numericFilterValues(query.filters)) }
    var picker by remember { mutableStateOf<String?>(null) }
    var choosing by remember { mutableStateOf<String?>(null) }
    var editingNumber by remember { mutableStateOf<String?>(null) }
    var invalid by remember { mutableStateOf(false) }
    Dialog(onDismissRequest = dismiss) {
        Surface(color = Plys.Surface900, shape = Plys.Large, border = androidx.compose.foundation.BorderStroke(1.dp, Plys.Hairline)) {
            Column(Modifier.widthIn(max = 760.dp).fillMaxWidth().heightIn(max = 480.dp).padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(stringResource(R.string.advanced_filters), style = MaterialTheme.typography.titleLarge)
                LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(categories) { category ->
                        Button(onClick = { picker = category }, modifier = Modifier.fillMaxWidth()) {
                            val selected = filterSelection(draft, category).firstOrNull()
                            val value = selected?.let { api.filterOptionLabel(if (category == "excludeGenres") "genres" else category, it) } ?: stringResource(R.string.no_filter)
                            Text("${filterLabel(category, query.kind)} : $value", maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                    }
                    items(choices) { key ->
                        Button(onClick = { choosing = key }, modifier = Modifier.fillMaxWidth()) {
                            val value = choiceValue(draft, key)?.let { choiceLabel(key, it) } ?: stringResource(R.string.no_filter)
                            Text("${filterLabel(key, query.kind)} : $value", maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                    }
                    items(numberGroups.filter { query.kind == CatalogKind.MOVIES || it.title != R.string.filter_group_runtime }) { group ->
                        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                            Text(stringResource(group.title), style = MaterialTheme.typography.titleSmall, color = Plys.TextStrong)
                            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                group.fields.forEach { (key, label) ->
                                    Button(onClick = { editingNumber = key }, modifier = Modifier.weight(1f)) {
                                        Text("${stringResource(label)} : ${numeric[key]?.takeIf { it.isNotBlank() } ?: stringResource(R.string.no_filter)}",
                                            maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    }
                                }
                            }
                        }
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
    choosing?.let { key ->
        ChoiceDialog(filterLabel(key, query.kind), { choosing = null }) {
            item { Choice(stringResource(R.string.no_filter), choiceValue(draft, key) == null) { draft = withChoice(draft, key, null); choosing = null } }
            items(if (key == "resolution") CatalogFilters.RESOLUTIONS else CatalogFilters.AUDIO_FORMATS) { value ->
                Choice(choiceLabel(key, value), choiceValue(draft, key) == value) { draft = withChoice(draft, key, value); choosing = null }
            }
        }
    }
    picker?.let { key ->
        FilterPickerDialog(query, key, draft, api, { picker = null }) { draft = withFilterSelection(draft, key, it); picker = null }
    }
    editingNumber?.let { key ->
        FilterInputDialog(filterLabel(key, query.kind), numeric[key].orEmpty(), if (key == "ratingMin") KeyboardType.Decimal else KeyboardType.Number,
            dismiss = { editingNumber = null },
            apply = { numeric = numeric + (key to it); invalid = false; editingNumber = null },
        )
    }
}

@Composable
private fun FilterInputDialog(title: String, initial: String, keyboardType: KeyboardType, dismiss: () -> Unit, apply: (String) -> Unit) {
    var value by remember { mutableStateOf(initial) }
    val keyboard = LocalSoftwareKeyboardController.current
    val close = { keyboard?.hide(); dismiss() }
    val save = { keyboard?.hide(); apply(value) }
    Dialog(onDismissRequest = close) {
        Surface(color = Plys.Surface900, shape = Plys.Large, border = androidx.compose.foundation.BorderStroke(1.dp, Plys.Hairline)) {
            Column(Modifier.widthIn(max = 600.dp).fillMaxWidth().padding(24.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(title, style = MaterialTheme.typography.titleLarge)
                OutlinedTextField(value, { value = it }, Modifier.fillMaxWidth().focusInitially(), singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = keyboardType, imeAction = ImeAction.Done),
                    keyboardActions = KeyboardActions(onDone = { save() }),
                )
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Action(R.string.cancel, onClick = close)
                    Action(R.string.apply, onClick = save)
                }
            }
        }
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
    val selected = remember { filterSelection(draft, category).take(1) }
    var editingSearch by remember { mutableStateOf(false) }
    var options by remember { mutableStateOf(emptyList<FilterOption>()) }
    var error by remember { mutableStateOf("") }
    var loading by remember { mutableStateOf(false) }
    val context = LocalContext.current
    val locale = filterLocale(LocalConfiguration.current.locales[0].language)
    val excluded = when (category) { "genres" -> filterSelection(draft, "excludeGenres"); "excludeGenres" -> filterSelection(draft, "genres"); else -> emptyList() }
    LaunchedEffect(search, locale) {
        delay(300)
        loading = true
        error = ""
        val apiCategory = if (category == "excludeGenres") "genres" else category
        try {
            val page = try { api.filterOptions(query.kind, apiCategory, search, query.libraryId, selected, locale) }
            // Servers released before pt-BR support reject that locale: fall back to English names.
            catch (failure: ClientException) { if (locale == "pt-BR") api.filterOptions(query.kind, apiCategory, search, query.libraryId, selected, "en") else throw failure }
            options = page.options[apiCategory].orEmpty()
        }
        catch (failure: ClientException) { error = context.errorMessage(failure) }
        finally { loading = false }
    }
    Dialog(onDismissRequest = dismiss) {
        Surface(color = Plys.Surface900, shape = Plys.Large, border = androidx.compose.foundation.BorderStroke(1.dp, Plys.Hairline)) {
            Column(Modifier.widthIn(max = 700.dp).fillMaxWidth().heightIn(max = 480.dp).padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(filterLabel(category, query.kind), style = MaterialTheme.typography.titleLarge)
                Button(onClick = { editingSearch = true }, modifier = Modifier.fillMaxWidth().focusInitially()) {
                    Text(if (search.isBlank()) stringResource(R.string.search) else search, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                if (loading) Text(stringResource(R.string.loading))
                if (error.isNotBlank()) Text(error, color = MaterialTheme.colorScheme.error)
                LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    item {
                        Button(onClick = { apply(emptyList()) }, modifier = Modifier.fillMaxWidth()) { Text(stringResource(R.string.no_filter)) }
                    }
                    if (!loading && options.isEmpty()) item { Text(stringResource(R.string.empty)) }
                    items(options, key = { it.id }) { option ->
                        Button(onClick = { apply(listOf(option.id)) },
                            enabled = option.id !in excluded, modifier = Modifier.fillMaxWidth()) {
                            RadioButton(selected = option.id in selected, onClick = null)
                            Text(option.label, modifier = Modifier.weight(1f))
                        }
                    }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Action(R.string.cancel, onClick = dismiss)
                }
            }
        }
    }
    if (editingSearch) FilterInputDialog(stringResource(R.string.search), search, KeyboardType.Text,
        dismiss = { editingSearch = false }, apply = { search = it; editingSearch = false },
    )
}

/** Maps the device language to a filter locale the server understands. */
internal fun filterLocale(language: String): String = when (language) {
    "fr" -> "fr"
    "pt" -> "pt-BR"
    else -> "en"
}
