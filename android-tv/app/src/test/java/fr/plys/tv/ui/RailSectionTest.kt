package fr.plys.tv.ui

import fr.plys.tv.data.HomeResponse
import fr.plys.tv.data.WatchlistResponse
import org.junit.Assert.assertEquals
import org.junit.Test

class RailSectionTest {
    @Test fun `the home screen is the home entry, even showing a search`() {
        assertEquals(Section.HOME, sectionOf(Screen.Home(HomeResponse())))
    }

    @Test fun `the search page is the search entry`() {
        assertEquals(Section.SEARCH, sectionOf(Screen.Search))
    }

    @Test fun `other screens keep their own entry`() {
        assertEquals(Section.WATCHLIST, sectionOf(Screen.Watchlist(WatchlistResponse())))
        assertEquals(Section.NONE, sectionOf(Screen.Login))
    }
}
