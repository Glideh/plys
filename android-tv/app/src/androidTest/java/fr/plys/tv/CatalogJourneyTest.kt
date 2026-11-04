package fr.plys.tv

import android.graphics.Bitmap
import android.view.KeyEvent
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsActions
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

    @Test fun homeKindsFiltersAndFocus() {
        val filtered = AtomicBoolean(false)
        MockWebServer().use { server ->
            server.dispatcher = object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    val url = request.requestUrl!!
                    val body = when (url.encodedPath) {
                        "/api/v1/setup/status" -> """{"needsSetup":false}"""
                        "/api/v1/auth/login", "/api/v1/auth/refresh" -> """{"accessToken":"fixture","refreshToken":"fixture-refresh","user":{"id":"viewer","username":"Viewer","locale":"en"}}"""
                        "/api/v1/server" -> """{"id":"local"}"""
                        "/api/v1/libraries" -> """{"items":[{"id":"library","name":"Living room","kind":"movies"}]}"""
                        "/api/v1/me/home" -> """{"continueWatching":[{"id":"film","kind":"movie","title":"Resume movie","positionSeconds":120,"durationSeconds":7200,"source":{"serverId":"local","serverName":"Home","itemId":"film"}}],"nextUp":[{"id":"episode","kind":"episode","title":"Episode two","showTitle":"A series","source":{"serverId":"local","itemId":"episode"}}]}"""
                        "/api/v1/kinds/movies/items" -> {
                            filtered.set(url.queryParameter("filters")?.contains("878") == true)
                            """{"items":[{"id":"film","kind":"movie","title":"Alien","year":1979}],"total":1,"offset":0,"limit":40}"""
                        }
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
            screenshot("home")
            assertTrue(rule.onNodeWithText(text(R.string.other)).getUnclippedBoundsInRoot().bottom <= rule.onRoot().getUnclippedBoundsInRoot().bottom)
            rule.onNodeWithText(text(R.string.movies)).performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            rule.onNodeWithText(text(R.string.movies)).performKeyInput { pressKey(Key.DirectionRight) }
            rule.onNodeWithText(text(R.string.series)).assertIsFocused()
            rule.onNodeWithText(text(R.string.movies)).activate()
            rule.waitUntil(10000) { rule.onAllNodesWithText("Alien").fetchSemanticsNodes().isNotEmpty() }
            screenshot("catalog")
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
            rule.onNodeWithText("Alien").activate()
            rule.waitUntil(10000) { rule.onAllNodesWithText(text(R.string.play)).fetchSemanticsNodes().isNotEmpty() }
            screenshot("detail")
            InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            rule.waitUntil(10000) { rule.onAllNodesWithText(rule.activity.getString(R.string.filters_count, 1)).fetchSemanticsNodes().isNotEmpty() }
            assertTrue(filtered.get())
        }
    }
}