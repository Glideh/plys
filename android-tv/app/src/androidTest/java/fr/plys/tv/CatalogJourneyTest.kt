package fr.plys.tv

import android.graphics.Bitmap
import android.view.KeyEvent
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.unit.height
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference
import android.view.View
import android.view.ViewGroup
import androidx.media3.ui.PlayerView
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.Assert.assertEquals
import org.json.JSONObject
import org.json.JSONArray
import androidx.compose.ui.text.TextLayoutResult
import androidx.activity.compose.setContent
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.lifecycle.ViewModelProvider
import fr.plys.tv.data.HomeItem
import fr.plys.tv.data.HomeResponse
import fr.plys.tv.data.CatalogKind
import fr.plys.tv.data.ServerRef
import fr.plys.tv.ui.CatalogViewModel
import fr.plys.tv.ui.CatalogState
import fr.plys.tv.ui.HomeScreen
import fr.plys.tv.ui.Screen
import fr.plys.tv.ui.PlysTheme
import java.io.ByteArrayOutputStream

@OptIn(ExperimentalTestApi::class)
class CatalogJourneyTest {
    @get:Rule val rule = createAndroidComposeRule<MainActivity>()

    private fun text(resource: Int) = rule.activity.getString(resource)

    private fun SemanticsNodeInteraction.activate() = performSemanticsAction(SemanticsActions.OnClick) { it() }

    private fun screenshot(name: String) {
        rule.waitForIdle()
        val image = if (name == "filters" || name == "filter-options" || name == "playback-subtitles") {
            InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot()
        } else rule.onRoot().captureToImage().asAndroidBitmap()
        File(rule.activity.getExternalFilesDir(null), "$name.png").outputStream().use { image.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test fun homeRowsStackVertically() {
        val data = HomeResponse(
            continueWatching = listOf(HomeItem("resume", "movie", "Resume fixture", ServerRef("local"))),
            nextUp = listOf(HomeItem("next", "episode", "Next fixture", ServerRef("local"))),
        )
        rule.runOnUiThread {
            val model = ViewModelProvider(rule.activity)[CatalogViewModel::class.java]
            rule.activity.setContent { PlysTheme { HomeScreen(data, model, emptyList(), emptyMap()) } }
        }
        val continued = rule.onNodeWithText(text(R.string.continue_watching)).getUnclippedBoundsInRoot()
        val next = rule.onNodeWithText(text(R.string.next_up)).getUnclippedBoundsInRoot()
        // Full-width rows, one under the other: up and down change row, left and right browse it.
        assertTrue(next.top > continued.bottom)
        assertEquals(continued.left, next.left)
        screenshot("home-rows")
    }

    @Test fun missingHomeStillShowsCatalogCounts() {
        MockWebServer().use { server ->
            server.dispatcher = object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    val path = request.requestUrl!!.encodedPath
                    val body = when {
                        path == "/api/v1/setup/status" -> """{"needsSetup":false}"""
                        path == "/api/v1/auth/login" -> """{"accessToken":"home-fixture","refreshToken":"home-refresh","user":{"id":"viewer","username":"Viewer","locale":"en"}}"""
                        path == "/api/v1/auth/logout" -> "{}"
                        path == "/api/v1/server" -> """{"id":"local"}"""
                        path == "/api/v1/libraries" -> """{"items":[{"id":"library","name":"Living room","kind":"movies"}]}"""
                        path.startsWith("/api/v1/kinds/") -> """{"items":[],"total":12,"offset":0,"limit":1}"""
                        else -> return MockResponse().setResponseCode(404)
                    }
                    return MockResponse().setHeader("Content-Type", "application/json").setBody(body)
                }
            }
            server.start()
            val serverAddress = server.url("/").toString()
            val model = ViewModelProvider(rule.activity)[CatalogViewModel::class.java]
            rule.waitUntil(10000) { !model.state.value.loading }
            rule.runOnIdle { model.login(serverAddress, "viewer", "fixture-password") }
            val count = rule.activity.resources.getQuantityString(R.plurals.title_count, 12, 12)
            rule.waitUntil(10000) { rule.onAllNodesWithText(count).fetchSemanticsNodes().size == 3 }
            rule.onNodeWithText(text(R.string.error_home_unavailable)).assertIsDisplayed()
            rule.onAllNodesWithText(text(R.string.error_not_found)).assertCountEquals(0)
            screenshot("home-api-unavailable")
            rule.runOnIdle { model.logout() }
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.sign_in)).fetchSemanticsNodes().isNotEmpty() }
        }
    }

    @Test fun signedInLaunchShowsSplashThenHome() {
        MockWebServer().use { server ->
            server.dispatcher = object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    val path = request.requestUrl!!.encodedPath
                    val body = when {
                        path == "/api/v1/setup/status" -> """{"needsSetup":false}"""
                        path == "/api/v1/auth/login" -> """{"accessToken":"splash-fixture","refreshToken":"splash-refresh","user":{"id":"viewer","username":"Viewer","locale":"en"}}"""
                        path == "/api/v1/auth/logout" -> "{}"
                        path == "/api/v1/server" -> """{"id":"local"}"""
                        path == "/api/v1/libraries" -> """{"items":[]}"""
                        path == "/api/v1/me/home" -> """{"continueWatching":[],"nextUp":[]}"""
                        path.startsWith("/api/v1/kinds/") -> """{"items":[],"total":0,"offset":0,"limit":1}"""
                        else -> return MockResponse().setResponseCode(404)
                    }
                    return MockResponse().setHeader("Content-Type", "application/json").setBody(body)
                }
            }
            server.start()
            val serverAddress = server.url("/").toString()
            val model = ViewModelProvider(rule.activity)[CatalogViewModel::class.java]
            rule.waitUntil(10000) { !model.state.value.loading }
            rule.runOnIdle { model.login(serverAddress, "viewer", "fixture-password") }
            rule.waitUntil(10000) { model.state.value.screen is Screen.Home && !model.state.value.loading }
            // A new launch with the session already held starts on the splash and goes straight home, never through the sign-in form.
            val screens = java.util.Collections.synchronizedList(mutableListOf<Screen>())
            val relaunched = AtomicReference<CatalogViewModel>()
            rule.runOnUiThread {
                val next = CatalogViewModel(rule.activity.application, androidx.lifecycle.SavedStateHandle())
                screens.add(next.state.value.screen)
                relaunched.set(next)
                rule.activity.setContent {
                    val state by next.state.collectAsState()
                    androidx.compose.runtime.SideEffect { screens.add(state.screen) }
                    PlysTheme { fr.plys.tv.ui.CatalogApp(next, state) }
                }
            }
            assertEquals(Screen.Splash, screens.first())
            rule.waitUntil(10000) { relaunched.get().state.value.screen is Screen.Home && !relaunched.get().state.value.loading }
            assertTrue(screens.none { it == Screen.Login })
            rule.onAllNodesWithText(text(R.string.sign_in)).assertCountEquals(0)
            rule.runOnIdle { model.logout() }
        }
    }

    @Test fun splashShowsTheBrandOnly() {
        rule.runOnUiThread { rule.activity.setContent { PlysTheme { fr.plys.tv.ui.SplashScreen() } } }
        val screenCenter = rule.onRoot().fetchSemanticsNode().boundsInRoot.center.y
        val logoBounds = rule.onNodeWithTag("splashLogo").fetchSemanticsNode().boundsInRoot
        val logoCenter = logoBounds.center.y
        assertEquals(screenCenter, logoCenter, 1f)
        val glowBounds = rule.onNodeWithTag("splashGlow").fetchSemanticsNode().boundsInRoot
        val expectedGlowY = logoCenter + logoBounds.height * (82f / 184f)
        assertEquals(expectedGlowY, glowBounds.center.y, 1f)
        val titleTop = rule.onNodeWithText(text(R.string.brand)).fetchSemanticsNode().boundsInRoot.top
        assertTrue(titleTop > logoBounds.bottom)
        rule.onNodeWithText(text(R.string.brand)).assertIsDisplayed()
        rule.onNodeWithText(text(R.string.splash_tagline)).assertIsDisplayed()
        rule.onAllNodesWithText(text(R.string.sign_in)).assertCountEquals(0)
        screenshot("splash")
    }

    @Test fun loginScreenShowsTheBrand() {
        val model = ViewModelProvider(rule.activity)[CatalogViewModel::class.java]
        rule.runOnUiThread {
            rule.activity.setContent {
                PlysTheme { fr.plys.tv.ui.CatalogApp(model, CatalogState(screen = Screen.Login)) }
            }
        }
        rule.onNodeWithText(text(R.string.brand)).assertIsDisplayed()
        rule.onNodeWithText(text(R.string.sign_in_title)).assertIsDisplayed()
    }

    @Test fun watchlistFromSideRailAndDetail() {
        val listed = AtomicBoolean(true)
        MockWebServer().use { server ->
            server.dispatcher = object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    val path = request.requestUrl!!.encodedPath
                    val body = when {
                        path == "/api/v1/setup/status" -> """{"needsSetup":false}"""
                        path == "/api/v1/auth/login" -> """{"accessToken":"watchlist-fixture","refreshToken":"watchlist-refresh","user":{"id":"viewer","username":"Viewer","locale":"en"}}"""
                        path == "/api/v1/auth/logout" -> "{}"
                        path == "/api/v1/server" -> """{"id":"local"}"""
                        path == "/api/v1/libraries" -> """{"items":[]}"""
                        path == "/api/v1/me/home" -> """{"continueWatching":[],"nextUp":[]}"""
                        path.startsWith("/api/v1/kinds/") -> """{"items":[],"total":0,"offset":0,"limit":1}"""
                        path == "/api/v1/me/watchlist" -> if (listed.get()) {
                            """{"items":[{"id":"film","kind":"movie","title":"Alien","year":1979,"addedAt":"2026-01-01T00:00:00Z","source":{"serverId":"local","serverName":"Home","itemId":"film"}}],"unavailableServers":0}"""
                        } else """{"items":[],"unavailableServers":0}"""
                        path == "/api/v1/items/film" -> """{"id":"film","kind":"movie","title":"Alien","year":1979,"inWatchlist":${listed.get()},"versions":[],"children":[]}"""
                        path == "/api/v1/items/film/watchlist" -> {
                            listed.set(request.method == "POST")
                            return MockResponse().setResponseCode(204)
                        }
                        else -> return MockResponse().setResponseCode(404)
                    }
                    return MockResponse().setHeader("Content-Type", "application/json").setBody(body)
                }
            }
            server.start()
            val serverAddress = server.url("/").toString()
            val model = ViewModelProvider(rule.activity)[CatalogViewModel::class.java]
            rule.waitUntil(10000) { !model.state.value.loading }
            rule.runOnIdle { model.login(serverAddress, "viewer", "fixture-password") }
            // Navigation is ignored while the home screen loads.
            rule.waitUntil(10000) { model.state.value.screen is Screen.Home && !model.state.value.loading }
            rule.onNodeWithContentDescription(text(R.string.watchlist)).activate()
            rule.waitUntil(10000) { rule.onAllNodesWithText("Alien").fetchSemanticsNodes().isNotEmpty() }
            screenshot("watchlist")
            rule.onNodeWithText("Alien").activate()
            rule.waitUntil(10000) { rule.onAllNodesWithContentDescription(text(R.string.remove_from_watchlist)).fetchSemanticsNodes().isNotEmpty() }
            rule.onNodeWithContentDescription(text(R.string.remove_from_watchlist)).activate()
            rule.waitUntil(10000) { rule.onAllNodesWithContentDescription(text(R.string.add_to_watchlist)).fetchSemanticsNodes().isNotEmpty() }
            assertTrue(!listed.get())
            // Going back reloads the watchlist, now without the removed title.
            rule.runOnIdle { model.back() }
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.watchlist_empty)).fetchSemanticsNodes().isNotEmpty() }
            rule.onAllNodesWithText("Alien").assertCountEquals(0)
            rule.runOnIdle { model.logout() }
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.sign_in)).fetchSemanticsNodes().isNotEmpty() }
        }
    }

    @Test fun startedTitleKeepsTheDetailLayout() {
        MockWebServer().use { server ->
            server.dispatcher = object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    val path = request.requestUrl!!.encodedPath
                    val versions = """[{"id":"version","label":"1080p","audio":[{"index":1,"language":"en","codec":"aac","channels":2,"default":true},{"index":2,"language":"fr","codec":"aac","channels":2}],"subtitles":[{"index":5,"language":"fr","codec":"subrip"}]}]"""
                    fun film(id: String, position: Int) = """{"id":"$id","kind":"movie","title":"Alien","overview":"A crew encounters an unknown life form.","year":1979,"runtimeMinutes":117,"positionSeconds":$position,"versions":$versions,"children":[]}"""
                    val body = when {
                        path == "/api/v1/setup/status" -> """{"needsSetup":false}"""
                        path == "/api/v1/auth/login" -> """{"accessToken":"started-fixture","refreshToken":"started-refresh","user":{"id":"viewer","username":"Viewer","locale":"en"}}"""
                        path == "/api/v1/auth/logout" -> "{}"
                        path == "/api/v1/server" -> """{"id":"local"}"""
                        path == "/api/v1/libraries" -> """{"items":[]}"""
                        path == "/api/v1/me/home" -> """{"continueWatching":[],"nextUp":[]}"""
                        path.startsWith("/api/v1/kinds/") -> """{"items":[],"total":0,"offset":0,"limit":1}"""
                        path == "/api/v1/items/fresh" -> film("fresh", 0)
                        path == "/api/v1/items/started" -> film("started", 3725)
                        else -> return MockResponse().setResponseCode(404)
                    }
                    return MockResponse().setHeader("Content-Type", "application/json").setBody(body)
                }
            }
            server.start()
            val serverAddress = server.url("/").toString()
            val model = ViewModelProvider(rule.activity)[CatalogViewModel::class.java]
            rule.waitUntil(10000) { !model.state.value.loading }
            rule.runOnIdle { model.login(serverAddress, "viewer", "fixture-password") }
            rule.waitUntil(10000) { model.state.value.screen is Screen.Home && !model.state.value.loading }
            fun open(id: String, play: String) {
                rule.runOnIdle { model.openItem(id) }
                rule.waitUntil(10000) { rule.onAllNodesWithText(play).fetchSemanticsNodes().isNotEmpty() }
            }
            open("fresh", text(R.string.play))
            val freshPlay = rule.onNodeWithText(text(R.string.play)).getUnclippedBoundsInRoot()
            val freshAudio = rule.onNodeWithContentDescription(text(R.string.audio)).getUnclippedBoundsInRoot()
            rule.onAllNodesWithContentDescription(text(R.string.restart)).assertCountEquals(0)
            rule.runOnIdle { model.back() }
            // Resuming adds a restart button: the actions stay on one line and the pickers below do not move.
            val resume = rule.activity.getString(R.string.resume, "1:02:05")
            open("started", resume)
            screenshot("detail-started")
            val play = rule.onNodeWithText(resume).getUnclippedBoundsInRoot()
            assertEquals(freshPlay.top, play.top)
            assertEquals(freshPlay.height, play.height)
            for (action in listOf(rule.onNodeWithContentDescription(text(R.string.restart)), rule.onNodeWithContentDescription(text(R.string.mark_watched)),
                rule.onNodeWithContentDescription(text(R.string.add_to_watchlist)))) {
                val bounds = action.assertIsDisplayed().getUnclippedBoundsInRoot()
                assertTrue(bounds.top >= play.top && bounds.bottom <= play.bottom)
            }
            assertEquals(freshAudio.top, rule.onNodeWithContentDescription(text(R.string.audio)).getUnclippedBoundsInRoot().top)
            // Left from the first action unfolds the rail; right goes back to that action, though the rail covers the page.
            val resumeButton = rule.onNodeWithText(resume)
            resumeButton.performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            resumeButton.performKeyInput { pressKey(Key.DirectionLeft) }
            rule.waitUntil(5000) { rule.onAllNodesWithText(text(R.string.sign_out)).fetchSemanticsNodes().isNotEmpty() }
            resumeButton.assertIsNotFocused()
            InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            rule.waitUntil(5000) { rule.onAllNodesWithText(text(R.string.sign_out)).fetchSemanticsNodes().isEmpty() }
            resumeButton.assertIsDisplayed()
            assertTrue(model.state.value.screen is Screen.Detail)
            rule.runOnIdle { model.logout() }
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.sign_in)).fetchSemanticsNodes().isNotEmpty() }
        }
    }

    @Test fun catalogSearchKeepsFocusAndDebouncesRequests() {
        val searchRequests = AtomicInteger()
        val lastSearch = AtomicReference("")
        MockWebServer().use { server ->
            server.dispatcher = object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    val url = request.requestUrl!!
                    if (url.encodedPath == "/api/v1/kinds/movies/items") {
                        url.queryParameter("query")?.takeIf(String::isNotBlank)?.let { query ->
                            lastSearch.set(query)
                            searchRequests.incrementAndGet()
                        }
                        return MockResponse().setHeader("Content-Type", "application/json")
                            .setBody("""{"items":[],"total":0,"offset":0,"limit":40}""")
                    }
                    val body = when (url.encodedPath) {
                        "/api/v1/setup/status" -> """{"needsSetup":false}"""
                        "/api/v1/auth/login" -> """{"accessToken":"search-fixture","refreshToken":"search-refresh","user":{"id":"viewer","username":"Viewer","locale":"en"}}"""
                        "/api/v1/auth/logout" -> "{}"
                        "/api/v1/server" -> """{"id":"local"}"""
                        "/api/v1/libraries" -> """{"items":[]}"""
                        "/api/v1/me/home" -> """{"continueWatching":[],"nextUp":[]}"""
                        else -> return MockResponse().setResponseCode(404)
                    }
                    return MockResponse().setHeader("Content-Type", "application/json").setBody(body)
                }
            }
            server.start()
            val serverAddress = server.url("/").toString()
            val model = ViewModelProvider(rule.activity)[CatalogViewModel::class.java]
            rule.waitUntil(10000) { !model.state.value.loading }
            rule.runOnIdle { model.login(serverAddress, "viewer", "fixture-password") }
            rule.waitUntil(10000) { model.state.value.screen is Screen.Home && !model.state.value.loading }
            rule.runOnIdle { model.openKind(CatalogKind.MOVIES) }
            rule.waitUntil(10000) { model.state.value.screen is Screen.Catalog && !model.state.value.loading }

            val searchLabel = text(R.string.search)
            val searchInput = rule.onNode(hasSetTextAction() and hasAnyAncestor(hasContentDescription(searchLabel)), useUnmergedTree = true)
            rule.onAllNodesWithContentDescription(searchLabel).onLast().activate()
            rule.waitUntil(5000) { searchInput.fetchSemanticsNode().config.getOrElse(SemanticsProperties.Focused) { false } }
            searchInput.performTextReplacement("Ali")
            searchInput.performTextReplacement("Alien")
            rule.waitUntil(5000) { searchRequests.get() > 0 }
            assertEquals(1, searchRequests.get())
            assertEquals("Alien", lastSearch.get())
            searchInput.assertIsFocused()

            searchInput.performTextInput("!")
            rule.waitUntil(5000) { searchRequests.get() == 2 }
            assertEquals("Alien!", lastSearch.get())
            searchInput.assertIsFocused()
            rule.runOnIdle { model.logout() }
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.sign_in)).fetchSemanticsNodes().isNotEmpty() }
        }
    }

    @Test fun homeKindsFiltersAndFocus() {
        val filtered = AtomicBoolean(false)
        val appended = AtomicBoolean(false)
        val playbackRequest = AtomicReference<JSONObject?>(null)
        val longTitle = "A title that is deliberately longer than the width of one television poster"
        val artwork = Bitmap.createBitmap(180, 270, Bitmap.Config.ARGB_8888).apply { eraseColor(android.graphics.Color.rgb(54, 91, 112)) }
        val imageBytes = ByteArrayOutputStream().also { artwork.compress(Bitmap.CompressFormat.PNG, 100, it) }.toByteArray()
        MockWebServer().use { server ->
            server.dispatcher = object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    val url = request.requestUrl!!
                    if (url.encodedPath == "/poster.png") return MockResponse().setHeader("Content-Type", "image/png").setBody(okio.Buffer().write(imageBytes))
                    if (url.encodedPath == "/api/v1/stream/session/subtitles/9.vtt") return MockResponse().setBody("WEBVTT\n\n00:00:00.000 --> 00:00:07.900\nSubtitle fixture\n")
                    if (url.encodedPath.startsWith("/api/v1/stream/session/")) {
                        val file = url.encodedPath.substringAfter("/api/v1/stream/session/")
                        if (!file.matches(Regex("(index\\.m3u8|init\\.mp4|seg_[0-9]{2}\\.m4s)"))) return MockResponse().setResponseCode(404)
                        val bytes = InstrumentationRegistry.getInstrumentation().context.assets.open("hls/$file").use { it.readBytes() }
                        return MockResponse().setHeader("Content-Type", if (file.endsWith("m3u8")) "application/vnd.apple.mpegurl" else "video/mp4").setBody(okio.Buffer().write(bytes))
                    }
                    val body = when (url.encodedPath) {
                        "/api/v1/setup/status" -> """{"needsSetup":false}"""
                        "/api/v1/auth/login", "/api/v1/auth/refresh" -> """{"accessToken":"fixture","refreshToken":"fixture-refresh","user":{"id":"viewer","username":"Viewer","locale":"en"}}"""
                        "/api/v1/auth/logout" -> "{}"
                        "/api/v1/server" -> """{"id":"local"}"""
                        "/api/v1/libraries" -> """{"items":[{"id":"library","name":"Living room","kind":"movies"}]}"""
                        "/api/v1/me/home" -> """{"continueWatching":[{"id":"film","kind":"movie","title":"Resume movie","positionSeconds":120,"durationSeconds":7200,"source":{"serverId":"local","serverName":"Home","itemId":"film"}}],"nextUp":[{"id":"episode","kind":"episode","title":"Episode two","showTitle":"A series","source":{"serverId":"local","itemId":"episode"}}]}"""
                        "/api/v1/kinds/movies/items" -> {
                            if (url.queryParameter("filters")?.contains("878") == true) filtered.set(true)
                            val offset = url.queryParameter("offset")?.toIntOrNull() ?: 0
                            val limit = url.queryParameter("limit")?.toIntOrNull() ?: 40
                            if (offset >= 40) appended.set(true)
                            val items = JSONArray()
                            for (index in offset until minOf(offset + limit, 65)) {
                                items.put(JSONObject().put("id", if (index == 0) "film" else "film-$index").put("kind", "movie")
                                    .put("title", when (index) { 0 -> "Alien"; 1 -> longTitle; else -> "Feature $index" })
                                    .put("year", 1979 + index).put("posterUrl", server.url("/poster.png").toString()))
                            }
                            JSONObject().put("items", items).put("total", 65).put("offset", offset).put("limit", limit).toString()
                        }
                        "/api/v1/kinds/series/items" -> """{"items":[],"total":0,"offset":0,"limit":1}"""
                        "/api/v1/kinds/other/items" -> """{"items":[],"total":2,"offset":0,"limit":1,"unavailableServers":1}"""
                        "/api/v1/kinds/movies/filters" -> """{"options":{"genres":[{"id":"878","label":"Science-fiction"},{"id":"18","label":"Drama"}]}}"""
                        "/api/v1/items/film" -> """{"id":"film","kind":"movie","title":"Alien","posterUrl":"/poster.png","overview":"A crew encounters an unknown life form.","year":1979,"cast":[{"name":"Sigourney Weaver","character":"Ripley"},{"name":"Tom Skerritt","character":"Dallas"}],"directors":["Ridley Scott"],"versions":[{"id":"version","label":"1080p","audio":[{"index":1,"language":"en","codec":"aac","channels":2,"default":true},{"index":2,"language":"fr","codec":"aac","channels":2}],"subtitles":[{"index":5,"language":"fr","codec":"subrip"}]},{"id":"alt","label":"720p","audio":[{"index":4,"language":"en","codec":"aac","channels":2,"default":true},{"index":7,"language":"fr","codec":"aac","channels":2}],"subtitles":[{"index":9,"language":"fr","codec":"subrip"},{"index":10,"language":"fr","codec":"hdmv_pgs_subtitle","imageBased":true}]}],"children":[]}"""
                        "/api/v1/playback/sessions" -> {
                            playbackRequest.set(JSONObject(request.body.readUtf8()))
                            """{"id":"session","playlistUrl":"/api/v1/stream/session/index.m3u8","start":0,"duration":8,"subtitleUrl":"/api/v1/stream/session/subtitles/9.vtt","burnedSubtitle":false}"""
                        }
                        "/api/v1/playback/sessions/session", "/api/v1/items/film/progress" -> "{}"
                        else -> return MockResponse().setResponseCode(404)
                    }
                    return MockResponse().setHeader("Content-Type", "application/json").setBody(body)
                }
            }
            server.start()
            rule.waitUntil(10000) { rule.onAllNodes(hasText(text(R.string.sign_in))).fetchSemanticsNodes().isNotEmpty() }
            rule.onNodeWithText(text(R.string.brand)).assertIsDisplayed()
            screenshot("login")
            rule.onNodeWithText(text(R.string.server)).performTextReplacement(server.url("/").toString())
            rule.onNodeWithText(text(R.string.username)).performTextReplacement("viewer")
            rule.onNodeWithText(text(R.string.password)).performTextReplacement("fixture-password")
            rule.onNodeWithText(text(R.string.sign_in)).activate()
            try {
                rule.waitUntil(10000) { rule.onAllNodesWithText("Resume movie").fetchSemanticsNodes().isNotEmpty() }
            } catch (failure: Throwable) {
                rule.onRoot().printToLog("PlysJourney")
                screenshot("failure")
                throw failure
            }
            val titleCount = { count: Int -> rule.activity.resources.getQuantityString(R.plurals.title_count, count, count) }
            val homeList = rule.onNode(hasScrollToNodeAction() and SemanticsMatcher.keyIsDefined(SemanticsProperties.VerticalScrollAxisRange))
            homeList.performScrollToNode(hasText(text(R.string.movies)))
            rule.waitUntil(10000) { rule.onAllNodesWithText(titleCount(65)).fetchSemanticsNodes().isNotEmpty() }
            rule.onNodeWithText(titleCount(0)).assertIsDisplayed()
            rule.waitUntil(10000) { rule.onAllNodesWithText("≥ ${titleCount(2)}").fetchSemanticsNodes().isNotEmpty() }
            rule.onNodeWithText(rule.activity.resources.getQuantityString(R.plurals.library_count, 1, 1)).assertIsDisplayed()
            screenshot("home")
            // The latest additions follow the libraries.
            homeList.performScrollToNode(hasText(text(R.string.recent_movies)))
            rule.onNodeWithText(text(R.string.recent_movies)).assertIsDisplayed()
            homeList.performScrollToNode(hasText(text(R.string.movies)))
            rule.onNodeWithText(text(R.string.other)).assertIsDisplayed()
            rule.onNodeWithText(text(R.string.movies)).performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            rule.onNodeWithText(text(R.string.movies)).performKeyInput { pressKey(Key.DirectionRight) }
            rule.onNodeWithText(text(R.string.series)).assertIsFocused()
            rule.onNodeWithText(text(R.string.movies)).activate()
            rule.waitUntil(10000) { rule.onAllNodesWithContentDescription(text(R.string.search)).fetchSemanticsNodes().isNotEmpty() }
            rule.waitUntil(10000) { rule.onAllNodesWithText("Alien").fetchSemanticsNodes().isNotEmpty() }
            rule.onAllNodesWithText("Plys TV").assertCountEquals(0)
            rule.onAllNodesWithText(text(R.string.sign_out)).assertCountEquals(0)
            rule.onAllNodesWithText(text(R.string.back)).assertCountEquals(0)
            val sortBounds = rule.onNodeWithText("${text(R.string.sort)} : ${text(R.string.sort_title)}").getUnclippedBoundsInRoot()
            val searchHeight = sortBounds.bottom - sortBounds.top
            val searchHeightPx = searchHeight.value * rule.activity.resources.displayMetrics.density
            val searchNodes = rule.onAllNodesWithContentDescription(text(R.string.search)).fetchSemanticsNodes()
            assertTrue(searchNodes.any { node ->
                kotlin.math.abs((node.boundsInRoot.bottom - node.boundsInRoot.top) - searchHeightPx) < 1f
            })
            screenshot("catalog")
            val layouts = mutableListOf<TextLayoutResult>()
            rule.onNodeWithText(longTitle).performSemanticsAction(SemanticsActions.GetTextLayoutResult) { it(layouts) }
            assertEquals(1, layouts.single().lineCount)
            rule.onAllNodesWithText(text(R.string.next_page)).assertCountEquals(0)
            val grid = rule.onNode(hasScrollToIndexAction() and SemanticsMatcher.keyIsDefined(SemanticsProperties.VerticalScrollAxisRange))
            grid.performScrollToIndex(35)
            rule.waitUntil(10000) { appended.get() }
            rule.waitUntil(10000) {
                grid.performScrollToIndex(55)
                rule.onAllNodesWithText("Feature 55").fetchSemanticsNodes().isNotEmpty()
            }
            screenshot("catalog-scrolled")
            // Down from the last row stays on the grid instead of reaching the rail's sign out button.
            grid.performScrollToIndex(64)
            rule.onNodeWithText("Feature 64").performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            rule.onNodeWithText("Feature 64").performKeyInput { pressKey(Key.DirectionDown) }
            rule.onNodeWithText("Feature 64").assertIsFocused()
            rule.onAllNodesWithText(text(R.string.sign_out)).assertCountEquals(0)
            grid.performScrollToIndex(0)
            // Up from the grid focuses the search without editing it; up again stays there; OK starts editing.
            val toolbarSearch = hasContentDescription(text(R.string.search)) and SemanticsMatcher("catalog toolbar search") { node ->
                kotlin.math.abs((node.boundsInRoot.bottom - node.boundsInRoot.top) - searchHeightPx) < 1f
            }
            val searchBox = rule.onNode(toolbarSearch)
            val searchInput = rule.onNode(hasSetTextAction() and hasAnyAncestor(hasContentDescription(text(R.string.search))), useUnmergedTree = true)
            rule.onNodeWithText("Alien").performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            rule.onNodeWithText("Alien").performKeyInput { pressKey(Key.DirectionUp) }
            searchBox.assertIsFocused()
            searchInput.assertIsNotFocused()
            searchBox.performKeyInput { pressKey(Key.DirectionUp) }
            searchBox.assertIsFocused()
            rule.onAllNodesWithText(text(R.string.sign_out)).assertCountEquals(0)
            searchBox.performKeyInput { pressKey(Key.DirectionCenter) }
            rule.waitUntil(5000) { searchInput.fetchSemanticsNode().config.getOrElse(SemanticsProperties.Focused) { false } }
            searchInput.performImeAction()
            searchInput.assertIsNotFocused()
            searchBox.assertIsFocused()
            rule.onNodeWithText(rule.activity.getString(R.string.filters_count, 0)).activate()
            val dialogInput = hasSetTextAction() and hasAnyAncestor(isDialog())
            rule.onAllNodes(dialogInput).assertCountEquals(0)
            val filterList = rule.onNode(hasScrollToIndexAction() and hasAnyAncestor(isDialog()))
            val numericLabel = text(R.string.filter_from)
            filterList.performScrollToNode(hasText(numericLabel))
            rule.onNodeWithText(numericLabel).performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            rule.onAllNodes(dialogInput).assertCountEquals(0)
            rule.onNodeWithText(numericLabel).activate()
            rule.onNode(dialogInput).performTextReplacement("2000")
            rule.onNode(dialogInput).performImeAction()
            rule.onAllNodes(dialogInput).assertCountEquals(0)
            val genreLabel = text(R.string.filter_genres)
            filterList.performScrollToNode(hasText(genreLabel))
            val genreBounds = rule.onNodeWithText(genreLabel).fetchSemanticsNode().boundsInRoot
            val excludedGenreBounds = rule.onNodeWithText(text(R.string.filter_excluded_genres)).fetchSemanticsNode().boundsInRoot
            assertEquals(genreBounds.top, excludedGenreBounds.top, 1f)
            assertTrue(excludedGenreBounds.left > genreBounds.right)
            screenshot("filters")
            rule.onNodeWithText(genreLabel).activate()
            rule.waitUntil(10000) { rule.onAllNodesWithText("Science-fiction").fetchSemanticsNodes().isNotEmpty() }
            // The picker's search field is passed over without opening the keyboard.
            rule.onAllNodes(dialogInput and isFocused()).assertCountEquals(0)
            screenshot("filter-options")
            rule.onNodeWithText("Science-fiction").activate()
            rule.onNodeWithText("${text(R.string.filter_genres)} : Science-fiction").activate()
            rule.waitUntil(10000) { rule.onAllNodesWithText("Drama").fetchSemanticsNodes().isNotEmpty() }
            rule.onNodeWithText("Drama").activate()
            rule.onNodeWithText("${text(R.string.filter_genres)} : Drama").activate()
            rule.waitUntil(10000) { rule.onAllNodesWithText("Science-fiction").fetchSemanticsNodes().isNotEmpty() }
            rule.onNodeWithText("Science-fiction").activate()
            rule.onAllNodesWithText(text(R.string.apply)).onLast().activate()
            rule.waitUntil(10000) { filtered.get() }
            rule.onNodeWithText(rule.activity.getString(R.string.filters_count, 2)).assertIsDisplayed()
            val firstFilterChip = rule.onNodeWithText(rule.activity.getString(
                R.string.remove_filter, "${text(R.string.filter_genres)}: Science-fiction",
            ))
            firstFilterChip.performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            firstFilterChip.assertIsFocused()
            val filterChipRow = rule.onNode(hasScrollToIndexAction() and SemanticsMatcher.keyIsDefined(SemanticsProperties.HorizontalScrollAxisRange))
            val chipBounds = firstFilterChip.getUnclippedBoundsInRoot()
            val rowBounds = filterChipRow.getUnclippedBoundsInRoot()
            assertTrue(chipBounds.left >= rowBounds.left && chipBounds.right <= rowBounds.right)
            rule.onNodeWithText("Alien").performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            assertTrue(rule.onNodeWithText("Alien").getUnclippedBoundsInRoot().bottom <= grid.getUnclippedBoundsInRoot().bottom)
            screenshot("catalog-focused")
            rule.onNodeWithText("Alien").performKeyInput { pressKey(Key.DirectionCenter) }
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.play)).fetchSemanticsNodes().isNotEmpty() }
            rule.onAllNodesWithText("Plys TV").assertCountEquals(0)
            rule.onAllNodesWithText(text(R.string.sign_out)).assertCountEquals(0)
            rule.onAllNodesWithText(text(R.string.back)).assertCountEquals(0)
            // The overview, the play button and the tracks fit above the fold; the cast follows when scrolling down.
            val detailList = rule.onNode(hasScrollToNodeAction() and SemanticsMatcher.keyIsDefined(SemanticsProperties.VerticalScrollAxisRange))
            rule.onNodeWithText("A crew encounters an unknown life form.").assertIsDisplayed()
            detailList.performScrollToNode(hasText("Sigourney Weaver"))
            rule.onNodeWithText("Sigourney Weaver").assertIsDisplayed()
            rule.onNodeWithText("Tom Skerritt").assertIsDisplayed()
            // Directors follow the cast, each in a disc; the page scrolls down to them.
            detailList.performScrollToNode(hasText("Ridley Scott"))
            rule.onNodeWithText(text(R.string.directors_title)).assertIsDisplayed()
            val director = rule.onNodeWithText("Ridley Scott")
            director.assertIsDisplayed()
            assertTrue(director.getUnclippedBoundsInRoot().top > rule.onNodeWithText("Sigourney Weaver").getUnclippedBoundsInRoot().bottom)
            detailList.performScrollToIndex(0)
            rule.onNodeWithContentDescription(text(R.string.version)).activate()
            rule.onNodeWithText("720p").activate()
            rule.onNodeWithContentDescription(text(R.string.audio)).activate()
            rule.onNodeWithText("fr · AAC · 2.0").activate()
            rule.onNodeWithContentDescription(text(R.string.subtitles)).activate()
            rule.onNodeWithText("fr · SUBRIP").activate()
            screenshot("detail")
            rule.onNodeWithText(text(R.string.play)).activate()
            rule.waitUntil(10000) { playbackRequest.get() != null }
            assertEquals("alt", playbackRequest.get()!!.getString("versionId"))
            assertEquals(7, playbackRequest.get()!!.getInt("audioIndex"))
            assertEquals(9, playbackRequest.get()!!.getInt("subtitleIndex"))
            var cueVisible = false
            var ready = false
            runCatching {
                rule.waitUntil(5000) {
                    rule.runOnIdle {
                        val player = findPlayer(rule.activity.window.decorView)?.player
                        ready = player?.playbackState == androidx.media3.common.Player.STATE_READY
                        cueVisible = player?.currentCues?.cues?.any { it.text.toString().contains("Subtitle fixture") } == true
                    }
                    ready
                }
            }
            if (ready) rule.runOnIdle { findPlayer(rule.activity.window.decorView)?.player?.pause() }
            rule.waitUntil(5000) { rule.onAllNodesWithContentDescription(text(R.string.forward)).fetchSemanticsNodes().isNotEmpty() }
            if (ready) {
                var receivedChannels = 0
                rule.runOnIdle { receivedChannels = (findPlayer(rule.activity.window.decorView)?.player as? androidx.media3.exoplayer.ExoPlayer)?.audioFormat?.channelCount ?: 0 }
                assertTrue(receivedChannels > 0)
                val controlsTop = rule.onNodeWithContentDescription(text(R.string.forward)).fetchSemanticsNode().boundsInWindow.top
                if (cueVisible) rule.runOnIdle {
                    val canvas = findPlayer(rule.activity.window.decorView)?.subtitleView?.getChildAt(0)
                    val position = IntArray(2)
                    canvas?.getLocationInWindow(position)
                    assertTrue(canvas != null && position[1] + canvas.height <= controlsTop)
                }
            }
            screenshot("playback-controls")
            rule.onNodeWithContentDescription(text(R.string.forward)).activate()
            rule.waitUntil(1000) { rule.onAllNodesWithText(rule.activity.getString(R.string.seek_by, "+10"), substring = true).fetchSemanticsNodes().isNotEmpty() }
            screenshot("playback-seek-preview")
            // There is no stop button any more: back closes the paused player.
            InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.play)).fetchSemanticsNodes().isNotEmpty() }
            InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            rule.waitUntil(10000) { rule.onAllNodesWithText(rule.activity.getString(R.string.filters_count, 2)).fetchSemanticsNodes().isNotEmpty() }
            assertTrue(filtered.get())
            val model = ViewModelProvider(rule.activity)[CatalogViewModel::class.java]
            rule.waitUntil(10000) { !model.state.value.loading }
            rule.runOnIdle { model.logout() }
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.sign_in)).fetchSemanticsNodes().isNotEmpty() }
        }
    }

    private fun findPlayer(view: View): PlayerView? {
        if (view is PlayerView) return view
        if (view is ViewGroup) for (index in 0 until view.childCount) findPlayer(view.getChildAt(index))?.let { return it }
        return null
    }
}