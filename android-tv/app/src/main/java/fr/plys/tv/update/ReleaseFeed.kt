package fr.plys.tv.update

import java.io.IOException
import kotlinx.serialization.SerialName
import kotlinx.serialization.SerializationException
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.OkHttpClient
import okhttp3.Request

@Serializable
data class GithubRelease(
    @SerialName("tag_name") val tagName: String,
    val draft: Boolean = false,
    val prerelease: Boolean = false,
    val assets: List<GithubAsset> = emptyList(),
)

@Serializable
data class GithubAsset(
    val name: String,
    @SerialName("browser_download_url") val url: String,
    val size: Long = 0,
)

/** A release newer than the installed app, with the APK to download. */
data class AvailableUpdate(val version: String, val apkUrl: String, val size: Long)

/** `x.y.z` (an optional leading `v` allowed) as numbers, or null for anything else. */
fun parseVersion(value: String): List<Int>? {
    val parts = value.trim().removePrefix("v").split('.')
    if (parts.size != 3) return null
    return parts.map { part -> part.toIntOrNull()?.takeIf { it >= 0 } ?: return null }
}

/** Only a strictly newer release is offered: never a downgrade, and nothing when either version is unreadable. */
fun isNewer(candidate: String, installed: String): Boolean {
    val next = parseVersion(candidate) ?: return false
    val current = parseVersion(installed) ?: return false
    for (index in next.indices) if (next[index] != current[index]) return next[index] > current[index]
    return false
}

fun availableUpdate(release: GithubRelease, installed: String): AvailableUpdate? {
    if (release.draft || release.prerelease || !isNewer(release.tagName, installed)) return null
    val apk = release.assets.firstOrNull { it.name.endsWith(".apk", ignoreCase = true) } ?: return null
    return AvailableUpdate(release.tagName.trim().removePrefix("v"), apk.url, apk.size)
}

private val releaseJson = Json { ignoreUnknownKeys = true }

fun parseRelease(body: String): GithubRelease? = try {
    releaseJson.decodeFromString(GithubRelease.serializer(), body)
} catch (_: SerializationException) {
    null
} catch (_: IllegalArgumentException) {
    null
}

/** What GitHub answered for the latest release. */
sealed interface FeedResult {
    /** A new body, with its ETag for the next conditional request. */
    data class Changed(val body: String, val etag: String?) : FeedResult
    /** 304: the stored body is still the latest release (and the request did not count against the rate limit). */
    data object NotModified : FeedResult
    /** No release yet, or the network or GitHub failed: the stored state is kept. */
    data object Unavailable : FeedResult
}

/** The `releases/latest` endpoint of the GitHub repository, read with `If-None-Match`. */
class ReleaseFeed(private val http: OkHttpClient, private val url: String, private val userAgent: String) {
    fun fetch(etag: String?): FeedResult {
        val request = Request.Builder().url(url)
            .header("Accept", "application/vnd.github+json")
            .header("X-GitHub-Api-Version", "2022-11-28")
            .header("User-Agent", userAgent)
            .apply { if (!etag.isNullOrBlank()) header("If-None-Match", etag) }
            .build()
        return try {
            http.newCall(request).execute().use { response ->
                when {
                    response.code == 304 -> FeedResult.NotModified
                    response.isSuccessful -> FeedResult.Changed(response.body?.string().orEmpty(), response.header("ETag"))
                    else -> FeedResult.Unavailable
                }
            }
        } catch (_: IOException) {
            FeedResult.Unavailable
        } catch (_: IllegalArgumentException) {
            FeedResult.Unavailable
        }
    }
}
