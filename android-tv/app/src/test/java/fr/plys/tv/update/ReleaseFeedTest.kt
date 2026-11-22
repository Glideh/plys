package fr.plys.tv.update

import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class ReleaseFeedTest {
    private lateinit var server: MockWebServer

    @Before fun start() {
        server = MockWebServer()
        server.start()
    }

    @After fun stop() = server.shutdown()

    private fun release(tag: String, vararg assets: String, draft: Boolean = false, prerelease: Boolean = false) =
        GithubRelease(tag, draft, prerelease, assets.map { GithubAsset(it, "https://example.test/$it", 1234) })

    @Test fun versionsCompareNumerically() {
        assertTrue(isNewer("0.7.10", "0.7.9"))
        assertTrue(isNewer("1.0.0", "0.99.99"))
        assertTrue(isNewer("v0.8.0", "0.7.4"))
        assertFalse(isNewer("0.7.4", "0.7.4"))
        assertFalse(isNewer("0.7.3", "0.7.4"))
        assertFalse(isNewer("0.8", "0.7.4"))
        assertFalse(isNewer("0.8.0-beta", "0.7.4"))
        assertFalse(isNewer("0.8.0", "dev"))
    }

    @Test fun onlyNewerStableReleasesWithAnApkAreOffered() {
        assertEquals(AvailableUpdate("0.7.5", "https://example.test/app-debug.apk", 1234),
            availableUpdate(release("0.7.5", "notes.txt", "app-debug.apk"), "0.7.4"))
        assertNull(availableUpdate(release("0.7.4", "app-debug.apk"), "0.7.4"))
        assertNull(availableUpdate(release("0.7.5", "notes.txt"), "0.7.4"))
        assertNull(availableUpdate(release("0.7.5", "app-debug.apk", draft = true), "0.7.4"))
        assertNull(availableUpdate(release("0.7.5", "app-debug.apk", prerelease = true), "0.7.4"))
    }

    @Test fun parsesTheGithubPayload() {
        val body = """{"tag_name":"0.7.5","draft":false,"prerelease":false,"name":"0.7.5",
            "assets":[{"name":"app-debug.apk","size":42,"browser_download_url":"https://github.com/o/r/releases/download/0.7.5/app-debug.apk"}]}"""
        val parsed = parseRelease(body)
        assertEquals("0.7.5", parsed?.tagName)
        assertEquals(42L, parsed?.assets?.single()?.size)
        assertNull(parseRelease("not json"))
        assertNull(parseRelease("""{"message":"Not Found"}"""))
    }

    @Test fun sendsTheStoredEtagAndReadsNotModified() {
        server.enqueue(MockResponse().setBody("""{"tag_name":"0.7.5"}""").setHeader("ETag", "\"abc\""))
        server.enqueue(MockResponse().setResponseCode(304))
        server.enqueue(MockResponse().setResponseCode(404))
        val feed = ReleaseFeed(OkHttpClient(), server.url("/repos/o/r/releases/latest").toString(), "Plys-TV/test")

        assertEquals(FeedResult.Changed("""{"tag_name":"0.7.5"}""", "\"abc\""), feed.fetch(null))
        val first = server.takeRequest()
        assertNull(first.getHeader("If-None-Match"))
        assertEquals("application/vnd.github+json", first.getHeader("Accept"))
        assertEquals("Plys-TV/test", first.getHeader("User-Agent"))

        assertEquals(FeedResult.NotModified, feed.fetch("\"abc\""))
        assertEquals("\"abc\"", server.takeRequest().getHeader("If-None-Match"))

        assertEquals(FeedResult.Unavailable, feed.fetch("\"abc\""))
    }

    @Test fun networkFailuresKeepTheStoredState() {
        val url = server.url("/latest").toString()
        server.shutdown()
        assertEquals(FeedResult.Unavailable, ReleaseFeed(OkHttpClient(), url, "Plys-TV/test").fetch(null))
    }
}
