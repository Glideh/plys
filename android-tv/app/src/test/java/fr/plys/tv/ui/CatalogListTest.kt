package fr.plys.tv.ui

import fr.plys.tv.data.CatalogKind
import fr.plys.tv.data.CatalogQuery
import fr.plys.tv.data.ItemPage
import fr.plys.tv.data.Library
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.SessionStore
import fr.plys.tv.data.StoredSession
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class CatalogListTest {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Unconfined)
    private val api = PlysApi(object : SessionStore {
        override fun read(): StoredSession? = null
        override fun server() = ""
        override fun save(session: StoredSession) {}
        override fun clear() {}
    })
    private val page = ItemPage(emptyList(), total = 0, offset = 0, limit = 40)
    private val library = Library("movies", "movies", "movies")

    @After fun teardown() { scope.cancel() }

    private fun catalog(query: CatalogQuery) = Screen.Catalog(library, page, query, CatalogList(scope, api, query, page))

    @Test fun backKeepsEachCatalogItsOwnList() {
        // Movies filtered on 1080p, a title opened, then an actor's movies: back must lead to the 1080p list, not the actor's.
        val filtered = catalog(CatalogQuery(CatalogKind.MOVIES, sort = "added"))
        val detail = Screen.Search
        val actor = catalog(CatalogQuery(CatalogKind.MOVIES, query = "actor"))
        assertEquals(setOf(filtered.list, actor.list), catalogListsInUse(listOf(Screen.Splash, filtered, detail), actor))
        // Once back from the actor's list, only the filtered one is still reachable.
        assertEquals(setOf(filtered.list), catalogListsInUse(listOf(Screen.Splash), filtered))
        assertEquals(emptySet<CatalogList>(), catalogListsInUse(emptyList(), Screen.Login))
    }

    @Test fun closingAListLeavesTheOthersOpen() {
        val first = CatalogList(scope, api, CatalogQuery(CatalogKind.MOVIES), page)
        val second = CatalogList(scope, api, CatalogQuery(CatalogKind.SERIES), page)
        first.positions["a"] = 3
        first.close()
        assertFalse(first.open)
        assertTrue(first.positions.isEmpty())
        assertTrue(second.open)
    }
}
