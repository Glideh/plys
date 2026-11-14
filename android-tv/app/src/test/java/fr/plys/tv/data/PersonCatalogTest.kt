package fr.plys.tv.data

import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class PersonCatalogTest {
    @Test fun `a person opens the catalog of the title kind filtered on them`() {
        assertEquals(CatalogQuery(CatalogKind.MOVIES, filters = CatalogFilters(actors = listOf(10205))), personCatalogQuery("movie", "actors", 10205))
        assertEquals(CatalogQuery(CatalogKind.SERIES, filters = CatalogFilters(creators = listOf(66633))), personCatalogQuery("series", "creators", 66633))
        assertEquals(CatalogKind.SERIES, personCatalogQuery("episode", "actors", 1).kind)
        assertEquals(CatalogKind.OTHER, personCatalogQuery("video", "actors", 1).kind)
    }

    @Test fun `the TMDB id of a person is optional`() {
        val json = Json { ignoreUnknownKeys = true }
        assertEquals(578, json.decodeFromString<CastMember>("""{"id":578,"name":"Ridley Scott","photoUrl":null}""").id)
        assertNull(json.decodeFromString<CastMember>("""{"name":"Older server","photoUrl":null}""").id)
    }
}
