package fr.plys.tv.data

import java.io.IOException
import java.time.Instant
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.suspendCancellableCoroutine
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
import okhttp3.Response
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

enum class Failure { URL, NETWORK, CREDENTIALS, SESSION, SETUP, BUSY, NOT_FOUND, STORAGE, RESPONSE, SERVER }

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
        .retryOnConnectionFailure(false)
        .build(),
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
    val signedIn: Boolean get() = session != null
    val savedServer: String get() = store.server()

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
        peerTokens.clear()
        filterLabels.clear()
        localServerId = null
        base = serverUrl(server)
        val setup = decode<SetupStatus>(request("setup/status", authenticated = false))
        if (setup.needsSetup) throw ClientException(Failure.SETUP)
        apply(decode<Session>(request(
            "auth/login", "POST", json.encodeToString(LoginRequest(username, password)), false,
        )))
    }

    suspend fun logout() = locked {
        val token = session?.refreshToken ?: store.read()?.refreshToken
        if (token != null) request("auth/logout", "POST", json.encodeToString(RefreshRequest(token)), false)
        store.clear()
        session = null
        peerTokens.clear()
        filterLabels.clear()
        localServerId = null
    }

    suspend fun libraries(): List<Library> = locked {
        decode<LibraryList>(request("libraries")).items
    }

    suspend fun items(libraryId: String, offset: Int): ItemPage = locked {
        decode(request("libraries/$libraryId/items?sort=title&offset=$offset&limit=40"))
    }

    suspend fun home(): HomeResponse = locked { decode(request("me/home")) }

    suspend fun catalog(query: CatalogQuery): ItemPage = locked {
        if (localServerId == null) localServerId = decode<ServerInfo>(request("server", authenticated = false)).id
        val params = checkNotNull(base).newBuilder().addQueryParameter("query", query.query)
            .addQueryParameter("sort", query.sort).addQueryParameter("offset", query.offset.toString())
            .addQueryParameter("limit", "40")
        query.libraryId?.let { params.addQueryParameter("library", it) }
        if (query.filters.count > 0) params.addQueryParameter("filters", Json.encodeToString(query.filters))
        val page = decode<ItemPage>(request("kinds/${query.kind.value}/items?${params.build().encodedQuery}"))
        page.copy(items = page.items.map { card -> card.copy(sources = card.sources.map { source ->
            if (source.serverId == localServerId) source.copy(serverUrl = null) else source
        }) })
    }

    suspend fun filterOptions(kind: CatalogKind, category: String, query: String, library: String?, selected: List<String>, locale: String): FilterOptions = locked {
        val params = checkNotNull(base).newBuilder().addQueryParameter("category", category)
            .addQueryParameter("query", query).addQueryParameter("locale", locale)
            .addQueryParameter("selected", selected.joinToString(","))
        library?.let { params.addQueryParameter("library", it) }
        decode<FilterOptions>(request("kinds/${kind.value}/filters?${params.build().encodedQuery}")).also { result ->
            result.options.forEach { (key, options) -> options.forEach { filterLabels["$key:${it.id}"] = it.label } }
        }
    }

    fun filterOptionLabel(category: String, id: String): String = filterLabels["$category:$id"] ?: id

    suspend fun item(id: String, source: ServerRef? = null): ItemDetail = locked { decode(sourceRequest(source, "items/$id")) }

    suspend fun start(versionId: String, position: Double, caps: ClientCaps, source: ServerRef? = null): PlaybackSession = locked {
        decode(sourceRequest(source, "playback/sessions", "POST", json.encodeToString(PlaybackRequest(versionId, position, caps))))
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

    private suspend fun execute(request: Request): Pair<Int, String> = suspendCancellableCoroutine { continuation ->
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
