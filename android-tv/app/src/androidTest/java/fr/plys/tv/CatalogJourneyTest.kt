package fr.plys.tv

import android.graphics.Bitmap
import android.view.KeyEvent
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File
import java.util.concurrent.atomic.AtomicBoolean
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
import androidx.compose.ui.unit.dp
import androidx.activity.compose.setContent
import androidx.lifecycle.ViewModelProvider
import fr.plys.tv.data.HomeItem
import fr.plys.tv.data.HomeResponse
import fr.plys.tv.data.ServerRef
import fr.plys.tv.ui.CatalogViewModel
import fr.plys.tv.ui.HomeScreen
import fr.plys.tv.ui.PlysTheme
import java.io.ByteArrayOutputStream

@OptIn(ExperimentalTestApi::class)
class CatalogJourneyTest {
    @get:Rule val rule = createAndroidComposeRule<MainActivity>()

    private fun text(resource: Int) = rule.activity.getString(resource)

    private fun SemanticsNodeInteraction.activate() = performSemanticsAction(SemanticsActions.OnClick) { it() }

    private fun screenshot(name: String) {
        rule.waitForIdle()
        val image = if (name == "filters" || name == "filter-options") {
            InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot()
        } else rule.onRoot().captureToImage().asAndroidBitmap()
        File(rule.activity.getExternalFilesDir(null), "$name.png").outputStream().use { image.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test fun homeRowsAdaptToAvailableWidth() {
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
        val root = rule.onRoot().getUnclippedBoundsInRoot()
        if (root.right - root.left >= 640.dp) {
            assertEquals(continued.top, next.top)
            assertTrue(next.left > continued.right)
        } else {
            assertTrue(next.top > continued.bottom)
        }
        screenshot("home-adaptive")
    }

    @Test fun homeKindsFiltersAndFocus() {
        val filtered = AtomicBoolean(false)
        val appended = AtomicBoolean(false)
        val longTitle = "A title that is deliberately longer than the width of one television poster"
        val artwork = Bitmap.createBitmap(180, 270, Bitmap.Config.ARGB_8888).apply { eraseColor(android.graphics.Color.rgb(54, 91, 112)) }
        val imageBytes = ByteArrayOutputStream().also { artwork.compress(Bitmap.CompressFormat.PNG, 100, it) }.toByteArray()
        MockWebServer().use { server ->
            server.dispatcher = object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    val url = request.requestUrl!!
                    if (url.encodedPath == "/poster.png") return MockResponse().setHeader("Content-Type", "image/png").setBody(okio.Buffer().write(imageBytes))
                    val body = when (url.encodedPath) {
                        "/api/v1/setup/status" -> """{"needsSetup":false}"""
                        "/api/v1/auth/login", "/api/v1/auth/refresh" -> """{"accessToken":"fixture","refreshToken":"fixture-refresh","user":{"id":"viewer","username":"Viewer","locale":"en"}}"""
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
                        "/api/v1/kinds/movies/filters" -> """{"options":{"genres":[{"id":"878","label":"Science-fiction"}]}}"""
                        "/api/v1/items/film" -> """{"id":"film","kind":"movie","title":"Alien","overview":"A crew encounters an unknown life form.","year":1979,"versions":[{"id":"version","label":"1080p"}],"children":[]}"""
                        else -> return MockResponse().setResponseCode(404)
                    }
                    return MockResponse().setHeader("Content-Type", "application/json").setBody(body)
                }
            }
            server.start()
            rule.waitUntil(10000) { rule.onAllNodes(hasText(text(R.string.sign_in))).fetchSemanticsNodes().isNotEmpty() }
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
            rule.waitUntil(10000) { rule.onAllNodesWithText(titleCount(65)).fetchSemanticsNodes().isNotEmpty() }
            rule.onNodeWithText(titleCount(0)).assertIsDisplayed()
            rule.waitUntil(10000) { rule.onAllNodesWithText("≥ ${titleCount(2)}").fetchSemanticsNodes().isNotEmpty() }
            rule.onNodeWithText(rule.activity.resources.getQuantityString(R.plurals.library_count, 1, 1)).assertIsDisplayed()
            screenshot("home")
            val continueBounds = rule.onNodeWithText(text(R.string.continue_watching)).getUnclippedBoundsInRoot()
            val nextBounds = rule.onNodeWithText(text(R.string.next_up)).getUnclippedBoundsInRoot()
            val rootBounds = rule.onRoot().getUnclippedBoundsInRoot()
            if (rootBounds.right - rootBounds.left >= 736.dp) {
                assertEquals(continueBounds.top, nextBounds.top)
                assertTrue(nextBounds.left > continueBounds.right)
            } else {
                assertTrue(nextBounds.top > continueBounds.bottom)
            }
            assertTrue(rule.onNodeWithText(text(R.string.other)).getUnclippedBoundsInRoot().bottom <= rule.onRoot().getUnclippedBoundsInRoot().bottom)
            rule.onNodeWithText(text(R.string.movies)).performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            rule.onNodeWithText(text(R.string.movies)).performKeyInput { pressKey(Key.DirectionRight) }
            rule.onNodeWithText(text(R.string.series)).assertIsFocused()
            rule.onNodeWithText(text(R.string.movies)).activate()
            rule.waitUntil(10000) { rule.onAllNodesWithText("Alien").fetchSemanticsNodes().isNotEmpty() }
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
            grid.performScrollToIndex(0)
            rule.onNodeWithText(rule.activity.getString(R.string.filters_count, 0)).activate()
            screenshot("filters")
            rule.onNodeWithText("${text(R.string.filter_genres)} (0)").activate()
            rule.waitUntil(10000) { rule.onAllNodesWithText("Science-fiction").fetchSemanticsNodes().isNotEmpty() }
            rule.onAllNodes(hasSetTextAction()).onLast().assertIsFocused()
            screenshot("filter-options")
            rule.onNodeWithText("Science-fiction").activate()
            rule.onAllNodesWithText(text(R.string.apply)).onLast().activate()
            rule.onAllNodesWithText(text(R.string.apply)).onLast().activate()
            rule.waitUntil(10000) { filtered.get() }
            rule.onNodeWithText(rule.activity.getString(R.string.filters_count, 1)).assertIsDisplayed()
            rule.onNodeWithText("Alien").performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            assertTrue(rule.onNodeWithText("Alien").getUnclippedBoundsInRoot().bottom <= grid.getUnclippedBoundsInRoot().bottom)
            screenshot("catalog-focused")
            rule.onNodeWithText("Alien").performKeyInput { pressKey(Key.DirectionCenter) }
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.play)).fetchSemanticsNodes().isNotEmpty() }
            screenshot("detail")
            InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            rule.waitUntil(10000) { rule.onAllNodesWithText(rule.activity.getString(R.string.filters_count, 1)).fetchSemanticsNodes().isNotEmpty() }
            assertTrue(filtered.get())
        }
    }
}