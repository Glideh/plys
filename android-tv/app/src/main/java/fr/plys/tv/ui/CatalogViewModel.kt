package fr.plys.tv.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.viewModelScope
import androidx.paging.Pager
import androidx.paging.PagingConfig
import androidx.paging.cachedIn
import fr.plys.tv.data.CatalogPagingSource
import fr.plys.tv.PlysApplication
import fr.plys.tv.data.ClientException
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.ItemPage
import fr.plys.tv.data.Library
import fr.plys.tv.data.CatalogKind
import fr.plys.tv.data.CatalogCount
import fr.plys.tv.data.CatalogQuery
import fr.plys.tv.data.HomeResponse
import fr.plys.tv.data.WatchlistItem
import fr.plys.tv.data.WatchlistResponse
import fr.plys.tv.data.ItemCard
import fr.plys.tv.data.ServerRef
import fr.plys.tv.data.TitleCopy
import fr.plys.tv.data.VersionChoice
import fr.plys.tv.data.MIN_SEARCH_LENGTH
import fr.plys.tv.data.SearchResult
import fr.plys.tv.data.PlaybackSelection
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.filterNotNull
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.launch
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import java.util.concurrent.ConcurrentHashMap

sealed interface Screen {
    data object Login : Screen
    data class Libraries(val items: List<Library>) : Screen
    data class Home(val data: HomeResponse) : Screen
    data class Watchlist(val data: WatchlistResponse) : Screen
    data class Catalog(val library: Library, val page: ItemPage, val query: CatalogQuery, val focusedId: String? = null, val focusedIndex: Int = 0) : Screen
    // target is the episode a series or season plays next: its versions drive the audio and subtitle choices.
    // copies are the items every server holds for a playable title, the shown one included: their versions are
    // offered together, and picking one held by another server shows that server's copy.
    data class Detail(
        val item: ItemDetail, val source: ServerRef? = null, val sources: List<ServerRef> = emptyList(),
        val selection: PlaybackSelection = PlaybackSelection(), val target: ItemDetail? = null,
        val copies: List<TitleCopy> = emptyList(),
    ) : Screen {
        val playable: ItemDetail get() = target ?: item

        /** Versions of the playable item across every server holding it, the shown copy's always up to date. */
        val versionChoices: List<VersionChoice> get() {
            val shown = TitleCopy(source, playable)
            val all = if (target != null || copies.isEmpty()) listOf(shown) else copies.map { if (isShown(it)) shown else it }
            return all.flatMap { copy -> copy.item.versions.map { VersionChoice(copy, it) } }
        }

        /** Series and seasons have no version to merge: they switch server as a whole. */
        val serverChoices: List<ServerRef> get() = if (item.children.isNotEmpty() && sources.size > 1) sources else emptyList()

        fun isShown(copy: TitleCopy): Boolean = copy.source?.serverId == source?.serverId && copy.item.id == item.id
    }
}

/** The global search of the home screen; [result] keeps the previous matches while a new search runs. */
data class SearchState(
    val query: String = "",
    val result: SearchResult? = null,
    val loading: Boolean = false,
    val error: ClientException? = null,
) {
    val term: String get() = query.trim()
    val active: Boolean get() = term.codePointCount(0, term.length) >= MIN_SEARCH_LENGTH
}

data class CatalogState(
    val screen: Screen = Screen.Login,
    val loading: Boolean = false,
    val error: ClientException? = null,
    val canRetry: Boolean = true,
    val playback: ItemDetail? = null,
    val playbackPosition: Double = 0.0,
    val playbackSource: ServerRef? = null,
    val playbackSelection: PlaybackSelection = PlaybackSelection(),
    val libraries: List<Library> = emptyList(),
    val catalogCounts: Map<CatalogKind, CatalogCount> = emptyMap(),
    /** Latest additions of each kind, shown on the home screen. */
    val recent: Map<CatalogKind, List<ItemCard>> = emptyMap(),
    val search: SearchState = SearchState(),
    /** Version of the signed-in server, empty until it answered. */
    val serverVersion: String = "",
)

private const val RECENT_ITEMS = 20
private const val SEARCH_DEBOUNCE_MS = 350L

@OptIn(ExperimentalCoroutinesApi::class)
class CatalogViewModel(application: Application, private val savedState: SavedStateHandle) : AndroidViewModel(application) {
    val api = (application as PlysApplication).api
    private val mutableState = MutableStateFlow(CatalogState())
    val state = mutableState.asStateFlow()
    private val history = mutableListOf<Screen>()
    private var retryAction: suspend () -> Unit = { restore() }
    private var activeJob: Job? = null
    private var countsJob: Job? = null
    private var searchJob: Job? = null
    private var generation = 0
    private data class PageRequest(val query: CatalogQuery, val seed: ItemPage, val revision: Int)
    private val pageRequest = MutableStateFlow<PageRequest?>(null)
    private val catalogPositions = ConcurrentHashMap<String, Int>()
    val catalogPages = pageRequest.filterNotNull().flatMapLatest { request ->
        Pager(PagingConfig(pageSize = 40, initialLoadSize = 40, prefetchDistance = 12, enablePlaceholders = false), initialKey = request.query.offset) {
            CatalogPagingSource(api, request.query, request.seed) { offset, cards ->
                cards.forEachIndexed { index, card -> catalogPositions[card.id] = offset + index }
            }
        }.flow
    }.cachedIn(viewModelScope)

    init { perform { restore() } }

    private suspend fun restore() {
        if (api.signedIn || api.restore()) {
            loadHome()
            savedState.get<String>("catalogQuery")?.let { raw ->
                val stored = runCatching { Json.decodeFromString<CatalogQuery>(raw) }.getOrNull()
                if (stored != null) {
                    val position = savedState.get<Int>("focusedPosition") ?: stored.offset
                    val query = stored.copy(offset = position / 40 * 40)
                    val page = api.catalog(query)
                    navigate(Screen.Catalog(Library(query.kind.value, query.kind.value, query.kind.value), page, query, savedState["focusedItem"], position - query.offset))
                    preparePaging(query, page)
                }
            }
        }
    }

    fun login(server: String, username: String, password: String) = perform(retryable = false) {
        countsJob?.cancel()
        clearSearch()
        mutableState.value = mutableState.value.copy(catalogCounts = emptyMap(), recent = emptyMap(), serverVersion = "")
        clearCatalogState()
        api.login(server, username, password)
        loadHome()
    }

    private suspend fun loadHome() {
        countsJob?.cancel()
        mutableState.value = mutableState.value.copy(libraries = api.libraries())
        mutableState.value = mutableState.value.copy(screen = Screen.Home(HomeResponse()))
        try {
            mutableState.value = mutableState.value.copy(screen = Screen.Home(api.home()))
        } catch (error: ClientException) {
            if (!api.signedIn) throw error
            mutableState.value = mutableState.value.copy(error = error, canRetry = true)
            retryAction = { loadHome() }
        }
        refreshCatalogCounts()
        refreshServerVersion()
    }

    /** Informative only: the rail simply shows nothing when the server does not answer. */
    private fun refreshServerVersion() {
        viewModelScope.launch {
            try {
                val version = api.serverInfo().version
                mutableState.value = mutableState.value.copy(serverVersion = version)
            } catch (_: ClientException) {}
        }
    }

    /** One request per kind gives both its title count and its latest additions. */
    private fun refreshCatalogCounts() {
        countsJob?.cancel()
        countsJob = viewModelScope.launch {
            for (kind in CatalogKind.entries) {
                try {
                    val page = api.catalog(CatalogQuery(kind, sort = "added"), limit = RECENT_ITEMS)
                    val state = mutableState.value
                    mutableState.value = state.copy(
                        catalogCounts = state.catalogCounts + (kind to CatalogCount(page.total, page.unavailableServers > 0)),
                        recent = state.recent + (kind to page.items),
                    )
                } catch (_: ClientException) {}
            }
        }
    }

    fun home() = perform { clearCatalogState(); clearSearch(); history.clear(); loadHome() }

    /**
     * Searches every kind while the viewer types, once the query is long enough; only the last query is answered.
     * [force] runs it again even when the query did not change, to retry after an error.
     */
    fun search(query: String, force: Boolean = false) {
        val current = mutableState.value.search
        val next = current.copy(query = query)
        if (!next.active) {
            searchJob?.cancel()
            mutableState.value = mutableState.value.copy(search = SearchState(query))
            return
        }
        if (!force && next.term == current.term && (current.loading || current.result != null)) {
            mutableState.value = mutableState.value.copy(search = next)
            return
        }
        searchJob?.cancel()
        mutableState.value = mutableState.value.copy(search = next.copy(loading = true, error = null))
        searchJob = viewModelScope.launch {
            delay(SEARCH_DEBOUNCE_MS)
            try {
                val result = api.search(next.term)
                mutableState.value = mutableState.value.copy(search = mutableState.value.search.copy(result = result, loading = false))
            } catch (error: ClientException) {
                if (!api.signedIn) {
                    history.clear()
                    mutableState.value = mutableState.value.copy(screen = Screen.Login, error = error, search = SearchState())
                } else {
                    mutableState.value = mutableState.value.copy(search = mutableState.value.search.copy(error = error, loading = false))
                }
            }
        }
    }

    fun clearSearch() = search("")

    fun openKind(kind: CatalogKind) = openCatalog(CatalogQuery(kind))

    /** The full listing of one kind for the current search, from the home screen. */
    fun openSearchKind(kind: CatalogKind) = openCatalog(CatalogQuery(kind, query = mutableState.value.search.term))

    private fun openCatalog(query: CatalogQuery) = perform {
        val kind = query.kind
        val page = api.catalog(query)
        navigate(Screen.Catalog(Library(kind.value, kind.value, kind.value), page, query))
        preparePaging(query, page)
        savedState["catalogQuery"] = Json.encodeToString(query)
    }

    fun openWatchlist() = perform { navigate(Screen.Watchlist(api.watchlist())) }

    fun openWatchlistItem(item: WatchlistItem) = openItem(item.source.itemId.ifBlank { item.id }, item.source)

    fun logout() = perform {
        countsJob?.cancel()
        clearSearch()
        mutableState.value = mutableState.value.copy(catalogCounts = emptyMap(), recent = emptyMap())
        api.logout()
        clearCatalogState()
        history.clear()
        mutableState.value = mutableState.value.copy(screen = Screen.Login)
    }

    fun openLibrary(library: Library) = perform {
        val query = CatalogQuery(CatalogKind.entries.first { it.value == library.kind }, libraryId = library.id)
        val page = api.catalog(query)
        navigate(Screen.Catalog(library, page, query))
        preparePaging(query, page)
    }

    fun changeQuery(query: CatalogQuery) = perform(replace = true) {
        val screen = mutableState.value.screen as? Screen.Catalog ?: return@perform
        val page = api.catalog(query)
        mutableState.value = mutableState.value.copy(screen = screen.copy(query = query, page = page, focusedId = null, focusedIndex = 0))
        preparePaging(query, page)
        savedState["catalogQuery"] = Json.encodeToString(query)
        savedState.remove<String>("focusedItem")
        savedState.remove<Int>("focusedPosition")
    }

    fun rememberFocus(id: String, index: Int = 0) {
        val screen = mutableState.value.screen as? Screen.Catalog ?: return
        if (screen.focusedId == id) return
        mutableState.value = mutableState.value.copy(screen = screen.copy(focusedId = id, focusedIndex = index))
        savedState["focusedItem"] = id
        savedState["focusedPosition"] = catalogPositions[id] ?: screen.query.offset + index
    }

    fun openCard(card: ItemCard) {
        val source = card.sources.firstOrNull()
        openItem(source?.itemId?.takeIf { it.isNotBlank() } ?: card.id, source, card.sources)
    }

    fun openItem(id: String, source: ServerRef? = null, sources: List<ServerRef> = emptyList()) = perform {
        val item = api.item(id, source)
        val copies = copies(item, source, sources)
        // A copy that cannot play is not shown first when another server's can.
        val shown = copies.takeIf { item.playableVersion == null }?.firstOrNull { it.item.playableVersion != null } ?: TitleCopy(source, item)
        navigate(Screen.Detail(shown.item, shown.source, sources, target = playTarget(shown.item, shown.source), copies = copies))
    }

    /** The items each server holds for a playable title, the given one first; servers that fail to answer are left out. */
    private suspend fun copies(item: ItemDetail, source: ServerRef?, sources: List<ServerRef>): List<TitleCopy> {
        if (item.children.isNotEmpty() || sources.size < 2) return emptyList()
        val others = sources
            .filterNot { it.serverId == source?.serverId && (it.itemId.isBlank() || it.itemId == item.id) }
            .filter { it.itemId.isNotBlank() }
            .distinctBy { it.serverId to it.itemId }
            .mapNotNull { other ->
                try { TitleCopy(other, api.item(other.itemId, other)) } catch (_: ClientException) { null }
            }
        return if (others.isEmpty()) emptyList() else listOf(TitleCopy(source, item)) + others
    }

    /** Shows a series or season as another server holds it. */
    fun selectSource(source: ServerRef) = perform {
        val screen = mutableState.value.screen as? Screen.Detail ?: return@perform
        val item = api.item(source.itemId, source)
        mutableState.value = mutableState.value.copy(screen = screen.copy(item = item, source = source, selection = PlaybackSelection(), target = playTarget(item, source)))
    }

    /** Picks a version; one held by another server shows that server's copy, with its own progress and watched state. */
    fun selectVersion(choice: VersionChoice) {
        val screen = mutableState.value.screen as? Screen.Detail ?: return
        val copy = choice.copy
        val next = if (screen.target != null || screen.isShown(copy)) screen
            else screen.copy(
                item = copy.item, source = copy.source,
                // Keeps what was loaded for the copy left, so coming back to it shows its latest state.
                copies = screen.copies.map { if (screen.isShown(it)) TitleCopy(screen.source, screen.item) else it },
            )
        mutableState.value = mutableState.value.copy(screen = next.copy(selection = PlaybackSelection(choice.version.id).normalized(next.playable)))
    }

    /** The episode a series or season would play; null for playable items or when it cannot be loaded. */
    private suspend fun playTarget(item: ItemDetail, source: ServerRef?): ItemDetail? {
        val target = item.playTarget?.takeIf { item.playableVersion == null && it.id != item.id } ?: return null
        return try { api.item(target.id, source) } catch (_: ClientException) { null }
    }

    private suspend fun refreshed(screen: Screen.Detail): Screen.Detail {
        val item = api.item(screen.item.id, screen.source)
        val target = playTarget(item, screen.source)
        // Keep the chosen tracks while the same episode stays next; a new episode follows them by language.
        val selection = when {
            target == null -> screen.selection.normalized(item)
            screen.target?.id == target.id -> screen.selection.normalized(target)
            screen.target != null -> screen.selection.forNext(screen.target, target)
            else -> PlaybackSelection().normalized(target)
        }
        return screen.copy(item = item, target = target, selection = selection)
    }

    fun selectPlayback(selection: PlaybackSelection) {
        val screen = mutableState.value.screen as? Screen.Detail ?: return
        mutableState.value = mutableState.value.copy(screen = screen.copy(selection = selection.normalized(screen.playable)))
    }

    fun toggleWatched() = perform {
        val screen = mutableState.value.screen as? Screen.Detail ?: return@perform
        api.setWatched(screen.item.id, !screen.item.watched, screen.source)
        mutableState.value = mutableState.value.copy(screen = refreshed(screen))
    }

    fun toggleWatchlist() = perform {
        val screen = mutableState.value.screen as? Screen.Detail ?: return@perform
        val listed = !screen.item.inWatchlist
        api.setWatchlisted(screen.item.id, listed, screen.source)
        mutableState.value = mutableState.value.copy(screen = screen.copy(item = screen.item.copy(inWatchlist = listed)))
    }

    fun play(item: ItemDetail, restart: Boolean = false) = perform {
        val target = item.playTarget
        val screen = mutableState.value.screen as? Screen.Detail
        val source = screen?.source
        val chosenFor = screen?.playable ?: item
        val playable = if (target == null) item
            else screen?.target?.takeIf { it.id == target.id } ?: api.item(target.id, source)
        val selection = screen?.selection ?: PlaybackSelection()
        if (playable.playableVersion == null) {
            throw ClientException(fr.plys.tv.data.Failure.NOT_FOUND)
        }
        mutableState.value = mutableState.value.copy(
            playback = playable,
            playbackPosition = if (restart) 0.0 else target?.positionSeconds ?: playable.positionSeconds,
            playbackSource = source,
            playbackSelection = if (playable.id == chosenFor.id) selection.normalized(playable) else selection.forNext(chosenFor, playable),
        )
    }

    fun closePlayer() {
        mutableState.value = mutableState.value.copy(playback = null)
        val screen = mutableState.value.screen
        if (screen is Screen.Detail) perform {
            mutableState.value = mutableState.value.copy(screen = refreshed(screen))
        }
    }

    fun retry() = perform(action = retryAction)

    fun back(): Boolean {
        if (mutableState.value.loading) return true
        if (history.isEmpty()) return false
        val screen = history.removeAt(history.lastIndex)
        if (screen is Screen.Home) clearCatalogState()
        mutableState.value = mutableState.value.copy(screen = screen, error = null)
        perform {
            val refreshed = when (screen) {
                Screen.Login -> screen
                is Screen.Home -> { loadHome(); mutableState.value.screen }
                is Screen.Libraries -> Screen.Libraries(api.libraries())
                is Screen.Watchlist -> Screen.Watchlist(api.watchlist())
                is Screen.Catalog -> screen
                is Screen.Detail -> refreshed(screen)
            }
            mutableState.value = mutableState.value.copy(screen = refreshed)
        }
        return true
    }

    val canGoBack: Boolean get() = history.isNotEmpty()

    private fun clearCatalogState() {
        savedState.remove<String>("catalogQuery")
        savedState.remove<String>("focusedItem")
        savedState.remove<Int>("focusedPosition")
        catalogPositions.clear()
    }

    private fun preparePaging(query: CatalogQuery, page: ItemPage) {
        catalogPositions.clear()
        pageRequest.value = PageRequest(query, page, generation)
    }

    private fun navigate(screen: Screen) {
        history.add(mutableState.value.screen)
        mutableState.value = mutableState.value.copy(screen = screen)
    }

    private fun perform(retryable: Boolean = true, replace: Boolean = false, action: suspend () -> Unit) {
        if (mutableState.value.loading && !replace) return
        if (mutableState.value.screen is Screen.Home) countsJob?.cancel()
        if (replace) activeJob?.cancel()
        val current = ++generation
        if (retryable) retryAction = action
        mutableState.value = mutableState.value.copy(loading = true, error = null, canRetry = retryable)
        activeJob = viewModelScope.launch {
            try {
                action()
            } catch (error: ClientException) {
                if (!api.signedIn) {
                    history.clear()
                    mutableState.value = mutableState.value.copy(screen = Screen.Login)
                } else if (mutableState.value.screen == Screen.Login) {
                    retryAction = { restore() }
                    mutableState.value = mutableState.value.copy(screen = Screen.Home(HomeResponse()), canRetry = true)
                }
                mutableState.value = mutableState.value.copy(error = error)
            } finally {
                if (generation == current) mutableState.value = mutableState.value.copy(loading = false)
            }
        }
    }
}
