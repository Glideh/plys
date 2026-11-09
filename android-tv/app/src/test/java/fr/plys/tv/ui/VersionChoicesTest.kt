package fr.plys.tv.ui

import fr.plys.tv.data.ChildItem
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.ServerRef
import fr.plys.tv.data.TitleCopy
import fr.plys.tv.data.Version
import org.junit.Assert.assertEquals
import org.junit.Test

class VersionChoicesTest {
    private val home = ServerRef("home", "Home", itemId = "a")
    private val peer = ServerRef("peer", "Peer", "https://peer.example", itemId = "b")

    private fun movie(id: String, vararg versions: String, watched: Boolean = false) =
        ItemDetail(id, "movie", "Film", versions = versions.map { Version(it, it) }, children = emptyList(), watched = watched)

    @Test fun `versions of every server holding the title are offered together`() {
        val local = movie("a", "a-1080", "a-2160")
        val remote = movie("b", "b-720")
        val screen = Screen.Detail(local, home, listOf(home, peer), copies = listOf(TitleCopy(home, local), TitleCopy(peer, remote)))

        assertEquals(listOf("home/a-1080", "home/a-2160", "peer/b-720"),
            screen.versionChoices.map { "${it.copy.source?.serverId}/${it.version.id}" })
        assertEquals(emptyList<ServerRef>(), screen.serverChoices)
    }

    @Test fun `the shown copy is offered as last loaded`() {
        val stale = movie("a", "a-1080")
        val refreshed = movie("a", "a-1080", watched = true)
        val screen = Screen.Detail(refreshed, home, listOf(home, peer), copies = listOf(TitleCopy(home, stale), TitleCopy(peer, movie("b", "b-720"))))

        assertEquals(true, screen.versionChoices.first().copy.item.watched)
    }

    @Test fun `a series switches server instead of merging versions`() {
        val series = ItemDetail("s", "series", "Show", versions = emptyList(), children = listOf(ChildItem("s1", "season", "Season 1")))
        val episode = movie("e", "e-1080").copy(kind = "episode")
        val screen = Screen.Detail(series, home, listOf(home, peer), target = episode)

        assertEquals(listOf("e-1080"), screen.versionChoices.map { it.version.id })
        assertEquals(listOf(home, peer), screen.serverChoices)
    }
}
