package fr.plys.tv.ui

import fr.plys.tv.data.HomeResponse
import fr.plys.tv.data.WatchlistResponse
import org.junit.Assert.assertEquals
import org.junit.Test

class RailSectionTest {
    @Test fun `the home screen without a search is the home entry`() {
        assertEquals(Section.HOME, sectionOf(Screen.Home(HomeResponse()), SearchState()))
    }

    @Test fun `the home screen with a search is the search entry`() {
        assertEquals(Section.SEARCH, sectionOf(Screen.Home(HomeResponse()), SearchState("ma")))
        assertEquals(Section.SEARCH, sectionOf(Screen.Home(HomeResponse()), SearchState("matrix")))
    }

    @Test fun `a search left on the home screen does not select the search entry elsewhere`() {
        assertEquals(Section.WATCHLIST, sectionOf(Screen.Watchlist(WatchlistResponse()), SearchState("matrix")))
        assertEquals(Section.NONE, sectionOf(Screen.Login, SearchState("matrix")))
    }
}
