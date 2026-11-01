package fr.plys.tv.data

import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.runBlocking
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test

class PlysApiTest {
    private lateinit var server: MockWebServer
    private val store = MemoryStore()
    private lateinit var api: PlysApi
    private val session = """{"accessToken":"access-1","refreshToken":"refresh-1","user":{"id":"u","username":"viewer","locale":"fr"},"expiresAt":"2030-01-01T00:00:00Z","tokenType":"Bearer"}"""

    @Before fun setup() {
        server = MockWebServer()
        server.start()
        api = PlysApi(store)
    }

    @After fun teardown() { server.shutdown() }

    private fun json(body: String, code: Int = 200) {
        server.enqueue(MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body))
    }

    private suspend fun login() {
        json("""{"needsSetup":false}""")
        json(session)
        api.login(server.url("/").toString(), "viewer", "password")
        server.takeRequest()
        val request = server.takeRequest()
        assertEquals("/api/v1/auth/login", request.path)
        assertTrue(request.body.readUtf8().contains("\"tokenDelivery\":\"body\""))
        assertNull(request.getHeader("Authorization"))
    }

    @Test fun `normalizes server roots and rejects unsafe addresses`() {
        assertEquals("https://example.test/", serverUrl(" https://example.test/api/v1/ ").toString())
        assertEquals("http://192.168.1.2:8096/", serverUrl("http://192.168.1.2:8096").toString())
        listOf("file:///tmp/video", "https://user:password@example.test", "https://example.test?q=1",
            "https://example.test/#fragment", "https://example.test/unsupported-prefix").forEach {
            assertEquals(Failure.URL, assertThrows(ClientException::class.java) { serverUrl(it) }.failure)
        }
    }

    @Test fun `login persists only server and refresh token`() = runBlocking {
        login()
        assertTrue(api.signedIn)
        assertEquals(StoredSession(server.url("/").toString(), "refresh-1"), store.value)
    }

    @Test fun `first run requires setup in web client`() = runBlocking {
        json("""{"needsSetup":true}""")
        try {
            api.login(server.url("/").toString(), "viewer", "password")
            fail("Expected setup error")
        } catch (error: ClientException) {
            assertEquals(Failure.SETUP, error.failure)
        }
        assertEquals(1, server.requestCount)
        assertNull(store.value)
    }

    @Test fun `concurrent requests rotate only once and retry with new access token`() = runBlocking {
        login()
        json("""{"code":"expired"}""", 401)
        json(session.replace("access-1", "access-2").replace("refresh-1", "refresh-2"))
        json("""{"items":[]}""")
        json("""{"items":[]}""")
        listOf(async { api.libraries() }, async { api.libraries() }).awaitAll()
        val requests = List(4) { server.takeRequest() }
        assertEquals(1, requests.count { it.path == "/api/v1/auth/refresh" })
        assertEquals("Bearer access-1", requests.first().getHeader("Authorization"))
        assertNull(requests[1].getHeader("Authorization"))
        assertEquals("""{"refreshToken":"refresh-1"}""", requests[1].body.readUtf8())
        requests.takeLast(2).forEach { assertEquals("Bearer access-2", it.getHeader("Authorization")) }
        assertEquals("refresh-2", store.value?.refreshToken)
    }

    @Test fun `rejected refresh clears session`() = runBlocking {
        store.value = StoredSession(server.url("/").toString(), "expired")
        json("""{"code":"invalid_refresh_token"}""", 401)
        try {
            api.restore()
            fail("Expected session error")
        } catch (error: ClientException) {
            assertEquals(Failure.SESSION, error.failure)
        }
        assertFalse(api.signedIn)
        assertNull(store.value)
    }

    @Test fun `network failure does not discard stored refresh token or retry rotation`() = runBlocking {
        store.value = StoredSession(server.url("/").toString(), "refresh-1")
        server.enqueue(MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AT_START))
        try {
            api.restore()
            fail("Expected network error")
        } catch (error: ClientException) {
            assertEquals(Failure.NETWORK, error.failure)
        }
        assertEquals("refresh-1", store.value?.refreshToken)
        assertEquals(1, server.requestCount)
    }

    @Test fun `storage failure does not retain an already rotated refresh token`() = runBlocking {
        store.value = StoredSession(server.url("/").toString(), "refresh-1")
        store.failWrites = true
        json(session.replace("refresh-1", "refresh-2"))
        try {
            api.restore()
            fail("Expected storage error")
        } catch (error: ClientException) {
            assertEquals(Failure.STORAGE, error.failure)
        }
        assertFalse(api.signedIn)
        assertNull(store.value)
    }

    @Test fun `catalog pagination and playback use API contracts`() = runBlocking {
        login()
        json("""{"items":[],"total":85,"offset":40,"limit":40}""")
        assertEquals(85, api.items("library", 40).total)
        assertEquals("/api/v1/libraries/library/items?sort=title&offset=40&limit=40", server.takeRequest().path)
        json("""{"id":"session","playlistUrl":"/api/v1/stream/session/index.m3u8","start":112,"duration":600}""", 201)
        val playback = api.start("version", 120.0, ClientCaps(listOf("h264"), listOf("aac")))
        assertEquals(112.0, playback.start, 0.0)
        val start = server.takeRequest()
        assertEquals("/api/v1/playback/sessions", start.path)
        val body = start.body.readUtf8()
        assertTrue(body.contains("\"versionId\":\"version\""))
        assertTrue(body.contains("\"start\":120.0"))
        assertTrue(body.contains("\"maxHeight\":1080"))
        assertTrue(api.mediaUrl(playback.playlistUrl).endsWith("/api/v1/stream/session/index.m3u8"))
        assertEquals(Failure.RESPONSE, assertThrows(ClientException::class.java) {
            api.mediaUrl("https://other.example/stream")
        }.failure)
        json("""{"watched":false}""")
        api.progress("item", 125.0, 600.0)
        assertEquals("""{"positionSeconds":125.0,"durationSeconds":600.0}""", server.takeRequest().body.readUtf8())
        json("", 204)
        api.stop("session")
        assertEquals("DELETE", server.takeRequest().method)
    }

    @Test fun `expired playback session can be stopped`() = runBlocking {
        login()
        json("""{"code":"not_found"}""", 404)
        api.stop("expired")
    }

    @Test fun `malformed catalog data is reported explicitly`() = runBlocking {
        login()
        json("""{"items":"wrong"}""")
        try {
            api.libraries()
            fail("Expected response error")
        } catch (error: ClientException) {
            assertEquals(Failure.RESPONSE, error.failure)
        }
    }

    private class MemoryStore : SessionStore {
        var value: StoredSession? = null
        var failWrites = false
        override fun read() = value
        override fun server() = value?.server ?: ""
        override fun save(session: StoredSession) {
            if (failWrites) throw ClientException(Failure.STORAGE)
            value = session
        }
        override fun clear() { value = null }
    }
}
