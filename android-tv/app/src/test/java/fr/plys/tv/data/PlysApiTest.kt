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
import androidx.paging.PagingSource
import fr.plys.tv.ui.choiceValue
import fr.plys.tv.ui.withChoice
import fr.plys.tv.ui.withFilterSelection
import fr.plys.tv.ui.audioClientCaps
import fr.plys.tv.ui.VideoDecoding
import fr.plys.tv.ui.videoClientCaps
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.int

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

    @Test fun `auto next preference is read from the session and saved on the server`() = runBlocking {
        login()
        assertFalse(api.autoplayNext)
        json("""{"id":"u","username":"viewer","locale":"fr","autoplayNext":true}""")
        assertTrue(api.setAutoplayNext(true))
        val request = server.takeRequest()
        assertEquals("PATCH", request.method)
        assertEquals("/api/v1/me", request.path)
        assertEquals("""{"autoplayNext":true}""", request.body.readUtf8())
        assertEquals("Bearer access-1", request.getHeader("Authorization"))
        assertTrue(api.autoplayNext)
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
        // Older servers refuse unknown fields: the external subtitle is only sent when chosen.
        assertFalse(body.contains("externalSubtitleId"))
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

    @Test fun `kind catalog encodes filters and normalizes local sources`() = runBlocking {
        login()
        json("""{"id":"origin"}""")
        json("""{"items":[{"id":"film","kind":"movie","title":"Alien","sources":[{"serverId":"origin","serverName":"Home","serverUrl":"${server.url("/")}","itemId":"film"}]}],"total":1,"offset":0,"limit":40,"unavailableServers":1}""")
        val page = api.catalog(CatalogQuery(CatalogKind.MOVIES, libraryId = "library", query = "Alien & space", sort = "rating", filters = CatalogFilters(genres = listOf(878), ratingMin = 7.0)))
        assertNull(page.items.single().sources.single().serverUrl)
        assertEquals(1, page.unavailableServers)
        assertEquals("/api/v1/server", server.takeRequest().requestUrl?.encodedPath)
        val request = server.takeRequest()
        assertEquals("/api/v1/kinds/movies/items", request.requestUrl?.encodedPath)
        assertEquals("Alien & space", request.requestUrl?.queryParameter("query"))
        assertEquals("library", request.requestUrl?.queryParameter("library"))
        assertEquals("rating", request.requestUrl?.queryParameter("sort"))
        assertEquals("""{"genres":[878],"ratingMin":7.0}""", request.requestUrl?.queryParameter("filters"))
        assertEquals("Bearer access-1", request.getHeader("Authorization"))
    }

    @Test fun `global search trims the query, normalizes local sources and skips unknown kinds`() = runBlocking {
        login()
        json("""{"id":"origin"}""")
        json("""{"groups":[{"kind":"movies","items":[{"id":"film","kind":"movie","title":"Alien","sources":[{"serverId":"origin","serverName":"Home","serverUrl":"https://public.example","itemId":"film"},{"serverId":"peer","serverName":"Friend","serverUrl":"https://peer.example","itemId":"remote"}]}],"total":3},{"kind":"music","items":[],"total":0}],"unavailableServers":2}""")
        val result = api.search("  Alien & co ", limit = 10)
        assertEquals(listOf(CatalogKind.MOVIES), result.groups.map { it.catalogKind })
        assertEquals(3, result.groups.single().total)
        assertEquals(listOf(null, "https://peer.example"), result.groups.single().items.single().sources.map { it.serverUrl })
        assertEquals(2, result.unavailableServers)
        assertEquals("/api/v1/server", server.takeRequest().requestUrl?.encodedPath)
        val request = server.takeRequest()
        assertEquals("/api/v1/search", request.requestUrl?.encodedPath)
        assertEquals("Alien & co", request.requestUrl?.queryParameter("query"))
        assertEquals("10", request.requestUrl?.queryParameter("limit"))
        assertEquals("Bearer access-1", request.getHeader("Authorization"))
    }

    @Test fun `filter suggestions preserve selection and locale`() = runBlocking {
        login()
        json("""{"options":{"actors":[{"id":"10205","label":"Sigourney Weaver"}]},"unavailableServers":0}""")
        assertEquals("Sigourney Weaver", api.filterOptions(CatalogKind.MOVIES, "actors", "Weaver & co", "library", listOf("10205"), "fr").options["actors"]?.single()?.label)
        val url = server.takeRequest().requestUrl!!
        assertEquals("/api/v1/kinds/movies/filters", url.encodedPath)
        assertEquals("Weaver & co", url.queryParameter("query"))
        assertEquals("10205", url.queryParameter("selected"))
        assertEquals("fr", url.queryParameter("locale"))
    }

    @Test fun `peer transport renews only federated token and never exposes origin credentials`() = runBlocking {
        login()
        MockWebServer().use { peer ->
            peer.start()
            val source = ServerRef("peer", "Other server", peer.url("/").toString(), "film")
            json("""{"token":"peer-1","expiresAt":"2099-01-01T00:00:00Z"}""")
            json("""{"token":"peer-2","expiresAt":"2099-01-01T00:00:00Z"}""")
            peer.enqueue(MockResponse().setResponseCode(401))
            peer.enqueue(MockResponse().setBody("""{"id":"film","kind":"movie","title":"Alien","versions":[],"children":[]}"""))
            assertEquals("Alien", api.item("film", source).title)
            repeat(2) {
                val originRequest = server.takeRequest()
                assertEquals("/api/v1/me/federation-token", originRequest.path)
                assertEquals("Bearer access-1", originRequest.getHeader("Authorization"))
                assertEquals("""{"audience":"peer"}""", originRequest.body.readUtf8())
            }
            assertEquals("Bearer peer-1", peer.takeRequest().getHeader("Authorization"))
            assertEquals("Bearer peer-2", peer.takeRequest().getHeader("Authorization"))
            assertTrue(api.signedIn)
            assertTrue(api.mediaUrl("/api/v1/stream/session/index.m3u8", source).startsWith(peer.url("/").toString()))
            assertEquals(Failure.RESPONSE, assertThrows(ClientException::class.java) { api.mediaUrl("https://third.example/stream", source) }.failure)
            peer.enqueue(MockResponse().setResponseCode(403))
            try { api.item("film", source); fail("Expected peer rejection") } catch (failure: ClientException) { assertEquals(Failure.SERVER, failure.failure) }
            assertTrue(api.signedIn)
            assertEquals("refresh-1", store.value?.refreshToken)
        }
    }

    @Test fun `advanced filters reject contradictory and invalid bounds`() {
        assertTrue(CatalogFilters(genres = listOf(878), yearMin = 2000, ratingMin = 7.0).valid)
        assertFalse(CatalogFilters(genres = listOf(27), excludeGenres = listOf(27)).valid)
        assertFalse(CatalogFilters(yearMin = 2020, yearMax = 2000).valid)
        assertFalse(CatalogFilters(runtimeMin = 120, runtimeMax = 90).valid)
        assertFalse(CatalogFilters(ratingMin = Double.NaN).valid)
        assertFalse(CatalogFilters(languages = listOf("english")).valid)
        assertEquals(2, CatalogFilters(genres = listOf(27, 878), ratingMin = 0.0).count)
        assertTrue(CatalogFilters(resolution = "2160p", audio = "atmos").valid)
        assertFalse(CatalogFilters(resolution = "4k").valid)
        assertFalse(CatalogFilters(audio = "dolby").valid)
        assertEquals(2, CatalogFilters(resolution = "1080p", audio = "dts").count)
    }

    @Test fun `paging reuses initial page and appends without losing filters`() = runBlocking {
        login()
        val query = CatalogQuery(CatalogKind.MOVIES, query = "space", filters = CatalogFilters(genres = listOf(878)))
        val seed = ItemPage(List(40) { ItemCard("item-$it", "movie", "Title $it") }, total = 41, offset = 0, limit = 40)
        val source = CatalogPagingSource(api, query, seed)
        val first = source.load(PagingSource.LoadParams.Refresh(key = 0, loadSize = 40, placeholdersEnabled = false)) as PagingSource.LoadResult.Page
        assertEquals(40, first.data.size)
        assertEquals(40, first.nextKey)
        assertNull(first.prevKey)
        assertEquals(2, server.requestCount)
        json("""{"id":"origin"}""")
        json("""{"items":[{"id":"item-40","kind":"movie","title":"Last title"}],"total":41,"offset":40,"limit":40}""")
        val appended = source.load(PagingSource.LoadParams.Append(key = 40, loadSize = 40, placeholdersEnabled = false)) as PagingSource.LoadResult.Page
        assertEquals("item-40", appended.data.single().id)
        assertNull(appended.nextKey)
        server.takeRequest()
        val request = server.takeRequest().requestUrl!!
        assertEquals("40", request.queryParameter("offset"))
        assertEquals("space", request.queryParameter("query"))
        assertEquals("""{"genres":[878]}""", request.queryParameter("filters"))
        json("""{"error":"unavailable"}""", 503)
        assertTrue(source.load(PagingSource.LoadParams.Append(key = 40, loadSize = 40, placeholdersEnabled = false)) is PagingSource.LoadResult.Error)
    }

    @Test fun `home counts use catalog totals with a single item request and indicate partial results`() = runBlocking {
        login()
        json("""{"id":"origin"}""")
        val cases = listOf(
            Triple(CatalogKind.MOVIES, 65, 0),
            Triple(CatalogKind.SERIES, 0, 0),
            Triple(CatalogKind.OTHER, 2, 1),
        )
        cases.forEachIndexed { index, (kind, total, unavailable) ->
            json("""{"items":[],"total":$total,"offset":0,"limit":1,"unavailableServers":$unavailable}""")
            assertEquals(CatalogCount(total, unavailable > 0), api.catalogCount(kind))
            if (index == 0) assertEquals("/api/v1/server", server.takeRequest().requestUrl?.encodedPath)
            val request = server.takeRequest().requestUrl!!
            assertEquals("/api/v1/kinds/${kind.value}/items", request.encodedPath)
            assertEquals("1", request.queryParameter("limit"))
            assertEquals("0", request.queryParameter("offset"))
            assertNull(request.queryParameter("library"))
            assertNull(request.queryParameter("filters"))
        }
    }

    @Test fun `missing home API is explicit and does not prevent title counts`() = runBlocking {
        login()
        json("""{"error":"not_found"}""", 404)
        try { api.home(); fail("Expected missing home API") }
        catch (error: ClientException) { assertEquals(Failure.HOME_UNAVAILABLE, error.failure) }
        assertTrue(api.signedIn)
        assertEquals("/api/v1/me/home", server.takeRequest().path)
        json("""{"id":"origin"}""")
        json("""{"items":[],"total":65,"offset":0,"limit":1}""")
        assertEquals(CatalogCount(65), api.catalogCount(CatalogKind.MOVIES))
        server.takeRequest()
        server.takeRequest()
        json("""{"error":"not_found"}""", 404)
        try { api.item("missing"); fail("Expected missing item") }
        catch (error: ClientException) { assertEquals(Failure.NOT_FOUND, error.failure) }
    }

    @Test fun `TV filter choices contain at most one value and replace previous choices`() {
        val first = withFilterSelection(CatalogFilters(), "genres", listOf("878", "18"))
        assertEquals(listOf(878), first.genres)
        assertEquals(listOf(18), withFilterSelection(first, "genres", listOf("18")).genres)
        assertTrue(withFilterSelection(first, "genres", emptyList()).genres.isEmpty())
        assertEquals(listOf("fr"), withFilterSelection(first, "languages", listOf("fr", "en")).languages)
        assertEquals(listOf(10205), withFilterSelection(first, "actors", listOf("10205", "578")).actors)
    }

    @Test fun `technical filters are encoded without the removed genre mode`() {
        val filters = withChoice(withChoice(withFilterSelection(CatalogFilters(), "genres", listOf("878")), "resolution", "1080p"), "audio", "dtsHdMa")
        assertEquals("""{"genres":[878],"resolution":"1080p","audio":"dtsHdMa"}""", Json.encodeToString(filters))
        assertNull(choiceValue(withChoice(filters, "audio", null), "audio"))
    }

    @Test fun `details decode cast directors and media tracks`() = runBlocking {
        login()
        json("""{"id":"film","kind":"movie","title":"Alien","cast":[{"name":"Sigourney Weaver","character":"Ripley","photoUrl":null}],"directors":["Ridley Scott"],"versions":[{"id":"version","label":"1080p","audio":[{"index":1,"language":"en","default":true},{"index":7,"language":"fr"}],"subtitles":[{"index":9,"language":"fr","codec":"subrip"}]}],"children":[]}""")
        val item = api.item("film")
        assertEquals("Sigourney Weaver", item.cast.single().name)
        assertEquals(listOf("Ridley Scott"), item.directors)
        assertEquals(7, item.versions.single().audio[1].index)
        assertEquals(9, item.versions.single().subtitles.single().index)
    }

    @Test fun `version changes discard invalid tracks and next episode preserves languages`() {
        val item = ItemDetail("film", "movie", "Film", versions = listOf(
            Version("first", "1080p", audio = listOf(MediaTrack(1, "en", default = true), MediaTrack(7, "fr")), subtitles = listOf(MediaTrack(9, "fr"))),
            Version("second", "720p", audio = listOf(MediaTrack(3, "en", default = true)), subtitles = listOf(MediaTrack(5, "en"))),
        ), children = emptyList())
        val french = PlaybackSelection("first", 7, 9).normalized(item)
        assertEquals(PlaybackSelection("first", 7, 9), french)
        assertEquals(PlaybackSelection("second", 3, null), PlaybackSelection("second", 7, 9).normalized(item))
        val next = item.copy(id = "next", versions = listOf(Version("next-version", "1080p", audio = listOf(MediaTrack(12, "en", default = true), MediaTrack(14, "fr")), subtitles = listOf(MediaTrack(17, "fr")))))
        assertEquals(PlaybackSelection("next-version", 14, 17), french.forNext(item, next))
        assertNull(french.copy(subtitleIndex = null).forNext(item, next).subtitleIndex)
    }

    @Test fun `downloaded subtitles are kept across versions and episodes by language`() {
        val item = ItemDetail("film", "movie", "Film", versions = listOf(
            Version("first", "1080p", subtitles = listOf(MediaTrack(9, "fre")), externalSubtitles = listOf(ExternalSubtitle("ext-fr", "fr"))),
            Version("second", "720p"),
        ), children = emptyList())
        val external = PlaybackSelection("first", null, null, "ext-fr").normalized(item)
        assertEquals("ext-fr", external.externalSubtitleId)
        assertNull(PlaybackSelection("first", null, 9, "ext-fr").normalized(item).externalSubtitleId)
        assertNull(PlaybackSelection("second", null, null, "ext-fr").normalized(item).externalSubtitleId)
        val next = item.copy(id = "next", versions = listOf(Version("next-version", "1080p", externalSubtitles = listOf(ExternalSubtitle("ext-en", "en"), ExternalSubtitle("ext-fr-2", "fr")))))
        assertEquals("ext-fr-2", external.forNext(item, next).externalSubtitleId)
    }

    @Test fun `online subtitle search and download use API contracts`() = runBlocking {
        login()
        json("""{"results":[{"fileId":42,"language":"fr","release":"Film.2020.1080p","downloadCount":3,"hashMatch":true,"trusted":false}]}""")
        val results = api.searchSubtitles("item", "version", "pt-BR")
        assertEquals(42, results.single().fileId)
        assertTrue(results.single().hashMatch)
        assertEquals("/api/v1/items/item/subtitles/search?versionId=version&language=pt-BR", server.takeRequest().path)
        json("""{"id":"sub","language":"fr","release":"Film.2020.1080p","hearingImpaired":false,"source":"opensubtitles"}""", 201)
        assertEquals("sub", api.downloadSubtitle("item", "version", results.single()).id)
        val body = Json.parseToJsonElement(server.takeRequest().body.readUtf8()).jsonObject
        assertEquals(42, body["fileId"]!!.jsonPrimitive.int)
        assertEquals("version", body["versionId"]!!.jsonPrimitive.content)
        json("""{"error":{"code":"subtitles_quota","message":"quota"}}""", 429)
        assertEquals(Failure.SUBTITLES_QUOTA, assertThrows(ClientException::class.java) { runBlocking { api.downloadSubtitle("item", "version", results.single()) } }.failure)
        json("""{"error":{"code":"subtitles_not_configured","message":"off"}}""", 409)
        assertEquals(Failure.SUBTITLES_ONLINE, assertThrows(ClientException::class.java) { runBlocking { api.searchSubtitles("item", "version", "fr") } }.failure)
        json("""{"id":"session","playlistUrl":"/api/v1/stream/session/index.m3u8","start":0,"duration":120,"subtitleUrl":"/api/v1/stream/session/subtitles/external.vtt"}""", 201)
        api.start("version", 0.0, ClientCaps(listOf("h264"), listOf("aac")), externalSubtitleId = "sub")
        server.takeRequest(); server.takeRequest()
        val start = Json.parseToJsonElement(server.takeRequest().body.readUtf8()).jsonObject
        assertEquals("sub", start["externalSubtitleId"]!!.jsonPrimitive.content)
    }

    @Test fun `playback sends the selected version audio and subtitle indices`() = runBlocking {
        login()
        json("""{"id":"session","playlistUrl":"/api/v1/stream/session/index.m3u8","start":60,"duration":120,"subtitleUrl":"/api/v1/stream/session/subtitles/9.vtt","burnedSubtitle":false}""", 201)
        val session = api.start("selected-version", 60.0, audioClientCaps(listOf("h264"), listOf("aac"), listOf("ac3")), audioIndex = 7, subtitleIndex = 9)
        val body = Json.parseToJsonElement(server.takeRequest().body.readUtf8()).jsonObject
        assertEquals("selected-version", body["versionId"]!!.jsonPrimitive.content)
        assertEquals(7, body["audioIndex"]!!.jsonPrimitive.int)
        assertEquals(9, body["subtitleIndex"]!!.jsonPrimitive.int)
        assertEquals("ac3", body["caps"]!!.jsonObject["preferredAudioCodec"]!!.jsonPrimitive.content)
        assertEquals("/api/v1/stream/session/subtitles/9.vtt", session.subtitleUrl)
        assertFalse(session.burnedSubtitle)
    }

    @Test fun `text subtitles wait for WebVTT and reject unrelated hosts`() = runBlocking {
        login()
        json("", 202)
        json("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nSubtitle fixture\n")
        val url = api.readySubtitleUrl("/api/v1/stream/session/subtitles/9.vtt")
        assertEquals(server.url("/api/v1/stream/session/subtitles/9.vtt").toString(), url)
        repeat(2) { assertNull(server.takeRequest().getHeader("Authorization")) }
        try { api.readySubtitleUrl("https://unrelated.example/subtitles.vtt"); fail("Expected origin rejection") }
        catch (error: ClientException) { assertEquals(Failure.RESPONSE, error.failure) }
    }

    @Test fun `audio capabilities include HDMI passthrough without requiring a Dolby decoder`() {
        val hdmi = audioClientCaps(listOf("h264"), listOf("aac"), listOf("ac3", "eac3"))
        assertEquals(listOf("aac", "ac3", "eac3"), hdmi.audioCodecs)
        assertEquals("ac3", hdmi.preferredAudioCodec)
        val stereo = audioClientCaps(listOf("h264"), listOf("aac", "ac3"), emptyList())
        assertEquals("", stereo.preferredAudioCodec)
        val plusOnly = audioClientCaps(listOf("h264"), listOf("aac"), listOf("eac3"))
        assertEquals("eac3", plusOnly.preferredAudioCodec)
        val ac3Only = audioClientCaps(listOf("h264"), listOf("aac", "ac3", "eac3"), listOf("ac3"))
        assertEquals(listOf("aac", "ac3"), ac3Only.audioCodecs)
    }

    @Test fun `video capabilities announce 4K and HDR only when every codec and the display handle them`() {
        val base = audioClientCaps(listOf("h264", "hevc"), listOf("aac"), emptyList())
        val uhd = listOf(VideoDecoding("h264", 2160, false), VideoDecoding("hevc", 2160, true))
        val hdrTv = videoClientCaps(base, uhd, displayHdr10 = true)
        assertEquals(2160, hdrTv.maxHeight)
        assertTrue(hdrTv.hdr)
        assertEquals(base.videoCodecs, hdrTv.videoCodecs)
        assertEquals(base.audioCodecs, hdrTv.audioCodecs)

        assertFalse(videoClientCaps(base, uhd, displayHdr10 = false).hdr)
        assertFalse(videoClientCaps(base, listOf(VideoDecoding("h264", 2160, false), VideoDecoding("hevc", 2160, false)), true).hdr)

        val fullHdH264 = videoClientCaps(base, listOf(VideoDecoding("h264", 1080, false), VideoDecoding("hevc", 2160, true)), true)
        assertEquals(1080, fullHdH264.maxHeight)
        assertEquals(1080, videoClientCaps(base, listOf(VideoDecoding("h264", 720, false), VideoDecoding("hevc", 720, false)), false).maxHeight)
        // A codec the device does not announce (AV1 in software only) does not lower the limit.
        assertEquals(2160, videoClientCaps(base, uhd + VideoDecoding("av1", 1080, false), true).maxHeight)
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
