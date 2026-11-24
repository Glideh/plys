package fr.plys.tv.data

import java.io.IOException
import java.time.Instant
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.delay
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Call
import okhttp3.Callback
import okhttp3.ConnectionPool
import okhttp3.Response
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

enum class Failure {
    URL, NETWORK, CREDENTIALS, SESSION, SETUP, DEVICE_LOGIN, BUSY, RATE_LIMITED, NOT_FOUND, HOME_UNAVAILABLE, SUBTITLES, SUBTITLES_QUOTA,
    SUBTITLES_ONLINE, STORAGE, RESPONSE, SERVER,
}

class ClientException(val failure: Failure, val status: Int = 0, cause: Throwable? = null) :
    Exception(failure.name, cause)

data class StoredSession(val server: String, val refreshToken: String)

interface SessionStore {
    fun read(): StoredSession?
    fun save(session: StoredSession)
    fun clear()
    fun server(): String
}

fun serverUrl(value: String): HttpUrl {
    val url = value.trim().toHttpUrlOrNull() ?: throw ClientException(Failure.URL)
    if (url.username.isNotEmpty() || url.password.isNotEmpty() || url.query != null || url.fragment != null) {
        throw ClientException(Failure.URL)
    }
    if (url.encodedPath.trimEnd('/') !in listOf("", "/api/v1")) throw ClientException(Failure.URL)
    return url.newBuilder().encodedPath("/").build()
}

class PlysApi(
    private val store: SessionStore,
    private val http: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .followRedirects(false)
        .followSslRedirects(false)
        // OkHttp would silently resend any request, refresh tokens included; idempotent ones are retried below.
        .retryOnConnectionFailure(false)
        // Idle connections outlive proxies by default (OkHttp keeps them 5 min, nginx closes them after 75 s):
        // the next request then fails on a connection the server already closed.
        .connectionPool(ConnectionPool(5, 30, TimeUnit.SECONDS))
        .build(),
    private val clock: () -> Long = System::currentTimeMillis,
) {
    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
    }
    private val mutex = Mutex()
    private var base: HttpUrl? = null
    private var session: Session? = null
    private var localServerId: String? = null
    private val peerTokens = mutableMapOf<String, FederatedToken>()
    private val filterLabels = mutableMapOf<String, String>()
    private val filterCache = mutableMapOf<String, Pair<Long, FilterOptions>>()
    val signedIn: Boolean get() = session != null
    val savedServer: String get() = store.server()
    /** The signed-in user's preference, as the server last returned it. */
    val autoplayNext: Boolean get() = session?.user?.autoplayNext ?: false

    // Rotation is serialized with all API calls; a refresh token must never be submitted twice.
    private suspend fun <T> locked(action: suspend () -> T): T = withContext(Dispatchers.IO) {
        mutex.withLock { action() }
    }

    suspend fun restore(): Boolean = locked {
        val stored = store.read() ?: return@locked false
        base = serverUrl(stored.server)
        rotate(stored.refreshToken)
        true
    }

    suspend fun login(server: String, username: String, password: String) = locked {
        connect(server)
        apply(decode<Session>(request(
            "auth/login", "POST", json.encodeToString(LoginRequest(username, password)), false,
        )))
    }

    /** Points the client at [server], forgetting what belonged to the previous one, and checks it is set up. */
    private suspend fun connect(server: String) {
        peerTokens.clear()
        filterLabels.clear()
        synchronized(filterCache) { filterCache.clear() }
        localServerId = null
        base = serverUrl(server)
        val setup = decode<SetupStatus>(request("setup/status", authenticated = false))
        if (setup.needsSetup) throw ClientException(Failure.SETUP)
    }

    /** Starts a sign-in approved from a phone; [pollDeviceLogin] then waits for the approval. */
    suspend fun startDeviceLogin(server: String): DeviceLogin = locked {
        connect(server)
        val start = decode<DeviceLoginStart>(request("auth/device", "POST", json.encodeToString(DeviceLoginRequest()), false))
        val origin = checkNotNull(base)
        // Without a public URL the phone opens the address the TV uses, which reaches the same server.
        val verification = start.verificationUrl.takeIf { it.isNotBlank() }?.toHttpUrlOrNull()
            ?: origin.newBuilder().addPathSegment("link").addQueryParameter("code", start.userCode).build()
        DeviceLogin(
            deviceCode = start.deviceCode,
            userCode = start.userCode,
            verificationUrl = verification.toString(),
            manualUrl = verification.newBuilder().query(null).build().toString(),
            intervalMillis = start.interval.coerceIn(1, 60) * 1000L,
        )
    }

    /**
     * Asks once whether the phone approved [login]: true once signed in, false while waiting.
     * Throws [Failure.DEVICE_LOGIN] when the request expired, was declined or already used: the server tells, so the
     * TV's own clock, often wrong, never decides.
     */
    suspend fun pollDeviceLogin(login: DeviceLogin): Boolean = locked {
        val body = try {
            request("auth/device/token", "POST", json.encodeToString(DeviceTokenRequest(login.deviceCode)), false)
        } catch (error: ClientException) {
            when (error.status) {
                409 -> return@locked false
                404 -> throw ClientException(Failure.DEVICE_LOGIN, error.status, error)
                else -> throw error
            }
        }
        apply(decode<Session>(body))
        true
    }

    suspend fun logout() = locked {
        val token = session?.refreshToken ?: store.read()?.refreshToken
        if (token != null) request("auth/logout", "POST", json.encodeToString(RefreshRequest(token)), false)
        store.clear()
        session = null
        peerTokens.clear()
        filterLabels.clear()
        synchronized(filterCache) { filterCache.clear() }
        localServerId = null
    }

    /** Saves the auto next preference on the server, where the web player reads it too, and returns the saved value. */
    suspend fun setAutoplayNext(enabled: Boolean): Boolean = locked {
        val user = decode<User>(request("me", "PATCH", json.encodeToString(AutoplayRequest(enabled))))
        session = session?.copy(user = user)
        user.autoplayNext
    }

    /** Public identity of the signed-in server, shown at the bottom of the side rail. */
    suspend fun serverInfo(): ServerInfo = locked {
        decode<ServerInfo>(request("server", authenticated = false)).also { localServerId = it.id }
    }

    suspend fun libraries(): List<Library> = locked {
        decode<LibraryList>(request("libraries")).items
    }

    suspend fun items(libraryId: String, offset: Int): ItemPage = locked {
        decode(request("libraries/$libraryId/items?sort=title&offset=$offset&limit=40"))
    }

    suspend fun home(): HomeResponse = locked {
        try { decode(request("me/home")) }
        catch (error: ClientException) {
            if (error.status == 404) throw ClientException(Failure.HOME_UNAVAILABLE, error.status, error)
            throw error
        }
    }

    /** The watchlist across the server and its peers; local entries come without a server URL, like the home rails. */
    suspend fun watchlist(): WatchlistResponse = locked { decode(request("me/watchlist")) }

    suspend fun setWatchlisted(id: String, listed: Boolean, source: ServerRef? = null) = locked {
        sourceRequest(source, "items/$id/watchlist", if (listed) "POST" else "DELETE")
        Unit
    }

    /** The copies of a title on the user's server and its paired servers, and whether any of them is in the watchlist. */
    suspend fun titleSources(kind: String, tmdbId: Int): TitleSources = locked {
        decode(request("kinds/$kind/titles/$tmdbId/sources"))
    }

    /** Whether any copy of a title is in the watchlist, on the user's server or a paired one. */
    suspend fun titleInWatchlist(kind: String, tmdbId: Int): Boolean = titleSources(kind, tmdbId).inWatchlist

    /** Removes a title from the watchlist on every server, whichever copy was added. */
    suspend fun removeTitleFromWatchlist(kind: String, tmdbId: Int) = locked {
        request("kinds/$kind/titles/$tmdbId/watchlist", "DELETE")
        Unit
    }

    suspend fun catalogCount(kind: CatalogKind): CatalogCount {
        val page = catalog(CatalogQuery(kind), limit = 1)
        return CatalogCount(page.total, page.unavailableServers > 0)
    }

    suspend fun catalog(query: CatalogQuery, limit: Int = 40): ItemPage = locked {
        if (localServerId == null) localServerId = decode<ServerInfo>(request("server", authenticated = false)).id
        val params = checkNotNull(base).newBuilder().addQueryParameter("query", query.query)
            .addQueryParameter("sort", query.sort).addQueryParameter("offset", query.offset.toString())
            .addQueryParameter("limit", limit.toString())
        query.libraryId?.let { params.addQueryParameter("library", it) }
        if (query.filters.count > 0) params.addQueryParameter("filters", Json.encodeToString(query.filters))
        val page = decode<ItemPage>(request("kinds/${query.kind.value}/items?${params.build().encodedQuery}"))
        page.copy(items = page.items.map(::withLocalSources))
    }

    /** Titles of every kind, on this server and its peers, whose title contains [query]; at most [limit] per kind. */
    suspend fun search(query: String, limit: Int = 20): SearchResult = locked {
        if (localServerId == null) localServerId = decode<ServerInfo>(request("server", authenticated = false)).id
        val params = checkNotNull(base).newBuilder().addQueryParameter("query", query.trim()).addQueryParameter("limit", limit.toString())
        val result = decode<SearchResult>(request("search?${params.build().encodedQuery}"))
        result.copy(groups = result.groups.filter { it.catalogKind != null }.map { group -> group.copy(items = group.items.map(::withLocalSources)) })
    }

    // The local server is reached through the address the user signed in with, not its advertised public one.
    private fun withLocalSources(card: ItemCard) = card.copy(sources = card.sources.map { source ->
        if (source.serverId == localServerId) source.copy(serverUrl = null) else source
    })

    suspend fun filterOptions(kind: CatalogKind, category: String, query: String, library: String?, selected: List<String>, locale: String): FilterOptions = locked {
        val params = checkNotNull(base).newBuilder().addQueryParameter("category", category)
            .addQueryParameter("query", query).addQueryParameter("locale", locale)
            .addQueryParameter("selected", selected.joinToString(","))
        library?.let { params.addQueryParameter("library", it) }
        val result = decode<FilterOptions>(request("kinds/${kind.value}/filters?${params.build().encodedQuery}"))
        // Older servers and peers name withdrawn country codes with an empty label.
        result.copy(options = result.options.mapValues { (_, options) -> namedFilterOptions(options) }).also { named ->
            named.options.forEach { (key, options) -> options.forEach { filterLabels["$key:${it.id}"] = it.label } }
        }
    }

    /**
     * The whole list of a short filter category, read through [load] and kept in memory for [FILTER_CACHE_MILLIS].
     * A list missing unavailable servers is not kept, so the next opening asks again.
     */
    suspend fun cachedFilterOptions(kind: CatalogKind, category: String, library: String?, locale: String, load: suspend () -> FilterOptions): FilterOptions {
        val key = listOf(kind.value, category, library.orEmpty(), locale).joinToString("\u0000")
        synchronized(filterCache) { filterCache[key]?.takeIf { clock() - it.first < FILTER_CACHE_MILLIS }?.let { return it.second } }
        return load().also { if (it.unavailableServers == 0) synchronized(filterCache) { filterCache[key] = clock() to it } }
    }

    fun filterOptionLabel(category: String, id: String): String = filterLabels["$category:$id"] ?: id

    /** Names a filter value picked outside the filter dialog, such as a person of a detail page. */
    fun rememberFilterLabel(category: String, id: String, label: String) { filterLabels["$category:$id"] = label }

    suspend fun item(id: String, source: ServerRef? = null): ItemDetail = locked { decode(sourceRequest(source, "items/$id")) }

    suspend fun start(
        versionId: String, position: Double, caps: ClientCaps, source: ServerRef? = null, audioIndex: Int? = null,
        subtitleIndex: Int? = null, externalSubtitleId: String? = null,
    ): PlaybackSession = locked {
        decode(sourceRequest(source, "playback/sessions", "POST",
            json.encodeToString(PlaybackRequest(versionId, position, caps, audioIndex, subtitleIndex, externalSubtitleId))))
    }

    /** Searches OpenSubtitles through the server holding the file. */
    suspend fun searchSubtitles(itemId: String, versionId: String, language: String, source: ServerRef? = null): List<SubtitleResult> = locked {
        onlineSubtitles {
            decode<SubtitleSearchResponse>(sourceRequest(source, "items/$itemId/subtitles/search?versionId=$versionId&language=$language")).results
        }
    }

    /** Stores a found subtitle on the server; it is then listed in the version's externalSubtitles. */
    suspend fun downloadSubtitle(itemId: String, versionId: String, result: SubtitleResult, source: ServerRef? = null): ExternalSubtitle = locked {
        onlineSubtitles {
            decode(sourceRequest(source, "items/$itemId/subtitles", "POST", json.encodeToString(
                SubtitleDownloadRequest(versionId, result.fileId, result.language, result.release, result.hearingImpaired))))
        }
    }

    // The server answers 429 once the OpenSubtitles quota is used up, 409 or 502 when the service is not usable.
    private inline fun <T> onlineSubtitles(action: () -> T): T = try {
        action()
    } catch (error: ClientException) {
        throw when (error.status) {
            429 -> ClientException(Failure.SUBTITLES_QUOTA, error.status, error)
            409, 502 -> ClientException(Failure.SUBTITLES_ONLINE, error.status, error)
            else -> error
        }
    }

    suspend fun stop(id: String, source: ServerRef? = null) = locked {
        try {
            sourceRequest(source, "playback/sessions/$id", "DELETE")
        } catch (error: ClientException) {
            // Expired sessions are already stopped; deletion is idempotent for the client.
            if (error.failure != Failure.NOT_FOUND) throw error
        }
        Unit
    }

    suspend fun setWatched(id: String, watched: Boolean, source: ServerRef? = null) = locked {
        sourceRequest(source, "items/$id/watched", if (watched) "POST" else "DELETE")
        Unit
    }

    suspend fun progress(id: String, position: Double, duration: Double, source: ServerRef? = null) = locked {
        sourceRequest(source, "items/$id/progress", "PUT", json.encodeToString(ProgressRequest(position, duration)))
        Unit
    }

    fun mediaUrl(path: String, source: ServerRef? = null): String {
        val origin = source?.serverUrl?.takeIf { it.isNotBlank() }?.let(::serverUrl) ?: base ?: throw ClientException(Failure.SESSION)
        val url = origin.resolve(path) ?: throw ClientException(Failure.RESPONSE)
        if (url.scheme != origin.scheme || url.host != origin.host || url.port != origin.port ||
            url.username.isNotEmpty() || url.password.isNotEmpty()) {
            throw ClientException(Failure.RESPONSE)
        }
        return url.toString()
    }

    fun posterUrl(path: String, source: ServerRef? = null): String {
        if (source != null) return mediaUrl(path, source)
        val absolute = path.toHttpUrlOrNull()
        if (absolute != null && absolute.username.isEmpty() && absolute.password.isEmpty()) return absolute.toString()
        return mediaUrl(path)
    }

    suspend fun readySubtitleUrl(path: String, source: ServerRef? = null): String {
        val url = mediaUrl(path, source)
        val ready = withTimeoutOrNull(60_000) {
            var available = false
            while (!available) {
                val (status, body) = execute(Request.Builder().url(url).build())
                when (status) {
                    200 -> {
                        if (!body.trimStart().startsWith("WEBVTT")) throw ClientException(Failure.SUBTITLES, status)
                        available = true
                    }
                    202 -> delay(500)
                    else -> throw ClientException(Failure.SUBTITLES, status)
                }
            }
            true
        }
        if (ready != true) throw ClientException(Failure.SUBTITLES)
        return url
    }

    private suspend fun sourceRequest(source: ServerRef?, path: String, method: String = "GET", body: String? = null, retried: Boolean = false): String {
        val address = source?.serverUrl?.takeIf { it.isNotBlank() } ?: return request(path, method, body)
        val peer = serverUrl(address)
        val cached = peerTokens[source.serverId]
        val valid = cached != null && runCatching { Instant.parse(cached.expiresAt).epochSecond - Instant.now().epochSecond > 30 }.getOrDefault(false)
        val token = if (valid) cached!! else decode<FederatedToken>(request("me/federation-token", "POST", json.encodeToString(mapOf("audience" to source.serverId)))).also { peerTokens[source.serverId] = it }
        val builder = Request.Builder().url(checkNotNull(peer.resolve("api/v1/$path")))
            .header("Accept", "application/json").header("Authorization", "Bearer ${token.token}")
        builder.method(method, when { body != null -> body.toRequestBody("application/json".toMediaType()); method == "POST" || method == "PUT" -> "".toRequestBody(); else -> null })
        val (status, text) = execute(builder.build())
        if (status == 401 && !retried) { peerTokens.remove(source.serverId); return sourceRequest(source, path, method, body, true) }
        if (status !in 200..299) throw ClientException(if (status == 404) Failure.NOT_FOUND else Failure.SERVER, status)
        return text
    }

    private inline fun <reified T> decode(body: String): T = try {
        json.decodeFromString<T>(body)
    } catch (error: SerializationException) {
        throw ClientException(Failure.RESPONSE, cause = error)
    }

    private fun apply(value: Session) {
        if (value.refreshToken.isBlank() || value.accessToken.isBlank()) throw ClientException(Failure.RESPONSE)
        try {
            store.save(StoredSession(checkNotNull(base).toString(), value.refreshToken))
        } catch (error: ClientException) {
            session = null
            store.clear()
            throw error
        }
        session = value
    }

    private suspend fun rotate(token: String) {
        try {
            apply(decode(request("auth/refresh", "POST", json.encodeToString(RefreshRequest(token)), false)))
        } catch (error: ClientException) {
            if (error.status == 401) {
                session = null
                store.clear()
                throw ClientException(Failure.SESSION)
            }
            throw error
        }
    }

    private suspend fun request(
        path: String,
        method: String = "GET",
        body: String? = null,
        authenticated: Boolean = true,
        retried: Boolean = false,
    ): String {
        val origin = base ?: throw ClientException(Failure.SESSION)
        val url = origin.resolve("api/v1/$path") ?: throw ClientException(Failure.URL)
        val builder = Request.Builder().url(url).header("Accept", "application/json")
        if (authenticated) {
            val token = session?.accessToken ?: throw ClientException(Failure.SESSION)
            builder.header("Authorization", "Bearer $token")
        }
        builder.method(method, when {
            body != null -> body.toRequestBody("application/json".toMediaType())
            method == "POST" || method == "PUT" -> "".toRequestBody()
            else -> null
        })
        val response = execute(builder.build())
        val (status, text) = response
        if (status == 401 && authenticated && !retried) {
            rotate(session?.refreshToken ?: throw ClientException(Failure.SESSION))
            return request(path, method, body, true, true)
        }
        if (status !in 200..299) {
            val failure = when (status) {
                401 -> if (authenticated) Failure.SESSION else Failure.CREDENTIALS
                404 -> Failure.NOT_FOUND
                429 -> Failure.RATE_LIMITED
                503 -> Failure.BUSY
                else -> Failure.SERVER
            }
            if (failure == Failure.SESSION) {
                session = null
                store.clear()
            }
            throw ClientException(failure, status)
        }
        return text
    }

    // A request that never reached the server, or one safe to repeat, is sent again once: a pooled connection
    // may have been closed by the server or a proxy just before being reused. POST is never repeated.
    private suspend fun execute(request: Request): Pair<Int, String> = try {
        send(request)
    } catch (error: ClientException) {
        if (error.failure != Failure.NETWORK || request.method !in setOf("GET", "PUT", "DELETE")) throw error
        send(request)
    }

    private suspend fun send(request: Request): Pair<Int, String> = suspendCancellableCoroutine { continuation ->
        val call = http.newCall(request)
        continuation.invokeOnCancellation { call.cancel() }
        call.enqueue(object : Callback {
            override fun onFailure(call: Call, error: IOException) {
                if (continuation.isActive) continuation.resumeWithException(ClientException(Failure.NETWORK, cause = error))
            }
            override fun onResponse(call: Call, response: Response) {
                try {
                    val result = response.use { it.code to (it.body?.string() ?: "") }
                    if (continuation.isActive) continuation.resume(result)
                } catch (error: IOException) {
                    if (continuation.isActive) continuation.resumeWithException(ClientException(Failure.NETWORK, cause = error))
                }
            }
        })
    }
}
