package fr.plys.tv.ui

import androidx.paging.Pager
import androidx.paging.PagingConfig
import androidx.paging.PagingData
import androidx.paging.cachedIn
import fr.plys.tv.data.CatalogPagingSource
import fr.plys.tv.data.CatalogQuery
import fr.plys.tv.data.ItemCard
import fr.plys.tv.data.ItemPage
import fr.plys.tv.data.PlysApi
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.isActive
import java.util.concurrent.ConcurrentHashMap

/**
 * The loaded pages of one catalog listing. Each catalog screen owns its own, kept in memory while the screen stays in
 * the history: back to it shows the titles it was left with, scrolled where they were, and not the last listing
 * opened since (a person's titles, opened from a detail page). It is closed once its screen leaves the history.
 */
class CatalogList(parent: CoroutineScope, api: PlysApi, val query: CatalogQuery, seed: ItemPage) {
    private val scope = CoroutineScope(parent.coroutineContext + SupervisorJob(parent.coroutineContext[Job]))

    /** Position of each loaded title in the whole listing, for the "42/9394" counter. */
    val positions = ConcurrentHashMap<String, Int>()

    val pages: Flow<PagingData<ItemCard>> =
        Pager(PagingConfig(pageSize = 40, initialLoadSize = 40, prefetchDistance = 12, enablePlaceholders = false), initialKey = query.offset) {
            CatalogPagingSource(api, query, seed) { offset, cards ->
                cards.forEachIndexed { index, card -> positions[card.id] = offset + index }
            }
        }.flow.cachedIn(scope)

    val open: Boolean get() = scope.isActive

    fun close() {
        scope.cancel()
        positions.clear()
    }
}

/** The catalog lists still reachable: the shown screen's and those of the screens back leads to. */
internal fun catalogListsInUse(history: List<Screen>, current: Screen): Set<CatalogList> =
    (history + current).mapNotNullTo(HashSet()) { (it as? Screen.Catalog)?.list }
