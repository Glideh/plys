package fr.plys.tv.ui

import fr.plys.tv.data.ItemDetail
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class PauseLogoTest {
    private val item = ItemDetail("a", "movie", "Title", logoUrl = "/api/v1/images/tmdb/w500/logo.png", versions = emptyList(), children = emptyList())
    private fun state(paused: Boolean = true, loading: Boolean = false, ended: Boolean = false, error: String? = null, logo: String? = item.logoUrl) =
        PlayerState(item.copy(logoUrl = logo), 0.0, 100.0, loading = loading, paused = paused, ended = ended, error = error)

    @Test fun `the logo shows while paused`() {
        assertEquals("/api/v1/images/tmdb/w500/logo.png", pauseLogo(state()))
    }

    @Test fun `no logo while playing, loading, ended, failed or when the title has none`() {
        assertNull(pauseLogo(state(paused = false)))
        assertNull(pauseLogo(state(loading = true)))
        assertNull(pauseLogo(state(ended = true)))
        assertNull(pauseLogo(state(error = "boom")))
        assertNull(pauseLogo(state(logo = null)))
        assertNull(pauseLogo(state(logo = "")))
    }
}
