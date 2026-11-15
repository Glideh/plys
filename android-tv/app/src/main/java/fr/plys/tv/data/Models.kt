package fr.plys.tv.data

import kotlinx.serialization.EncodeDefault
import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerialName

@Serializable
data class User(
    val id: String,
    val username: String,
    val locale: String,
    /** Starts the next episode when one ends; shared with the web player. */
    val autoplayNext: Boolean = false,
)

@Serializable
data class Session(val accessToken: String, val refreshToken: String, val user: User)

@Serializable
data class Library(val id: String, val name: String, val kind: String)

@Serializable
data class LibraryList(val items: List<Library>)

@Serializable
data class ItemCard(
    val id: String,
    val kind: String,
    val title: String,
    val year: Int? = null,
    val posterUrl: String? = null,
    val watched: Boolean = false,
    val rating: Double? = null,
    val sources: List<ServerRef> = emptyList(),
    val tmdbId: Int? = null,
)

@Serializable
data class ItemPage(val items: List<ItemCard>, val total: Int, val offset: Int, val limit: Int, val unavailableServers: Int = 0,
    val unavailableServerNames: List<String> = emptyList(),
)

/** The best matches of one library kind; total counts every matching title. */
@Serializable
data class SearchGroup(val kind: String, val items: List<ItemCard> = emptyList(), val total: Int = 0) {
    val catalogKind: CatalogKind? get() = CatalogKind.entries.firstOrNull { it.value == kind }
}

@Serializable
data class SearchResult(val groups: List<SearchGroup> = emptyList(), val unavailableServers: Int = 0, val unavailableServerNames: List<String> = emptyList())

/** Searches shorter than this, once trimmed, are rejected by the API. */
const val MIN_SEARCH_LENGTH = 3

data class CatalogCount(val total: Int, val partial: Boolean = false)

@Serializable
enum class CatalogKind(val value: String) {
    @SerialName("movies") MOVIES("movies"),
    @SerialName("series") SERIES("series"),
    @SerialName("other") OTHER("other"),
}

@Serializable
data class ServerRef(val serverId: String, val serverName: String = "", val serverUrl: String? = null, val itemId: String = "")

@Serializable
data class HomeItem(
    val id: String, val kind: String, val title: String,
    val source: ServerRef,
    val showTitle: String = "", val seasonNumber: Int? = null, val episodeNumber: Int? = null,
    val posterUrl: String? = null, val positionSeconds: Double = 0.0,
    val durationSeconds: Double = 0.0,
)

@Serializable
data class WatchlistItem(
    val id: String, val kind: String, val title: String,
    val source: ServerRef,
    val year: Int? = null, val posterUrl: String? = null,
)

@Serializable
data class WatchlistResponse(val items: List<WatchlistItem> = emptyList(), val unavailableServers: Int = 0, val unavailableServerNames: List<String> = emptyList())

@Serializable
data class HomeResponse(
    val continueWatching: List<HomeItem> = emptyList(),
    val nextUp: List<HomeItem> = emptyList(),
    val unavailableServers: Int = 0,
    val unavailableServerNames: List<String> = emptyList(),
)

@Serializable
data class CatalogFilters(
    val genres: List<Int> = emptyList(), val excludeGenres: List<Int> = emptyList(),
    val actors: List<Int> = emptyList(),
    val creators: List<Int> = emptyList(), val keywords: List<Int> = emptyList(),
    val languages: List<String> = emptyList(), val countries: List<String> = emptyList(),
    val yearMin: Int? = null, val yearMax: Int? = null,
    val ratingMin: Double? = null, val votesMin: Int? = null,
    val runtimeMin: Int? = null, val runtimeMax: Int? = null,
    val resolution: String? = null, val audio: String? = null,
) {
    val count: Int get() = listOf(genres, excludeGenres, actors, creators, keywords, languages, countries).count { it.isNotEmpty() } +
        listOf(yearMin, yearMax, ratingMin, votesMin, runtimeMin, runtimeMax, resolution, audio).count { it != null }
    val valid: Boolean get() = genres.none { it in excludeGenres } &&
        listOf(genres, excludeGenres, actors, creators, keywords).all { it.size <= 20 && it.all { id -> id > 0 } } &&
        listOf(languages, countries).all { it.size <= 20 && it.all { code -> code.matches(Regex("[a-zA-Z]{2}")) } } &&
        listOfNotNull(yearMin, yearMax).all { it in 1800..9999 } &&
        (yearMin == null || yearMax == null || yearMin <= yearMax) &&
        (runtimeMin == null || runtimeMax == null || runtimeMin <= runtimeMax) &&
        listOfNotNull(runtimeMin, runtimeMax).all { it in 0..10000 } &&
        (ratingMin == null || ratingMin.isFinite() && ratingMin in 0.0..10.0) && (votesMin == null || votesMin >= 0) &&
        (resolution == null || resolution in RESOLUTIONS) && (audio == null || audio in AUDIO_FORMATS)

    companion object {
        /** Values accepted by the API for the technical filters (see CatalogFilters in docs/openapi.yaml). */
        val RESOLUTIONS = listOf("2160p", "1080p", "720p", "SD")
        val AUDIO_FORMATS = listOf("atmos", "truehd", "eac3", "ac3", "dtsHdMa", "dts", "flac", "pcm", "aac", "opus", "mp3")
    }
}

@Serializable
data class CatalogQuery(
    val kind: CatalogKind, val libraryId: String? = null,
    val query: String = "", val sort: String = "title",
    val filters: CatalogFilters = CatalogFilters(), val offset: Int = 0,
)

@Serializable
data class FilterOption(val id: String, val label: String)

@Serializable
data class FilterOptions(val options: Map<String, List<FilterOption>> = emptyMap(), val unavailableServers: Int = 0, val unavailableServerNames: List<String> = emptyList())

@Serializable
data class ServerInfo(val id: String, val version: String = "")

@Serializable
data class FederatedToken(val token: String, val expiresAt: String)

@Serializable
data class Version(
    val id: String, val label: String, val durationSeconds: Double? = null, val error: String? = null,
    val audio: List<MediaTrack> = emptyList(), val subtitles: List<MediaTrack> = emptyList(),
    val resolution: String = "", val hdr: Boolean = false, val videoCodec: String = "",
    /** File size in bytes. */
    val size: Long = 0,
    /** Subtitles downloaded online for this file; absent from older servers. */
    val externalSubtitles: List<ExternalSubtitle> = emptyList(),
)

/** A subtitle downloaded from OpenSubtitles; the language is an OpenSubtitles code ("fr", "pt-BR"). */
@Serializable
data class ExternalSubtitle(val id: String, val language: String, val release: String = "", val hearingImpaired: Boolean = false)

/** A subtitle found online; hashMatch means it was made for this exact file. */
@Serializable
data class SubtitleResult(
    val fileId: Int, val language: String, val release: String = "", val downloadCount: Int = 0,
    val hearingImpaired: Boolean = false, val machineTranslated: Boolean = false, val hashMatch: Boolean = false,
)

@Serializable
data class SubtitleSearchResponse(val results: List<SubtitleResult> = emptyList())

@Serializable
data class SubtitleDownloadRequest(val versionId: String, val fileId: Int, val language: String, val release: String, val hearingImpaired: Boolean)

/** The item a server holds for a title, with the server it is read from (null for the signed-in server). */
data class TitleCopy(val source: ServerRef?, val item: ItemDetail)

/** A version of a title the user can pick, with the copy holding it. */
data class VersionChoice(val copy: TitleCopy, val version: Version)

@Serializable
data class MediaTrack(
    val index: Int, val language: String = "", val title: String = "", val codec: String = "",
    val channels: Int = 0, val default: Boolean = false, val forced: Boolean = false, val imageBased: Boolean = false,
)

@Serializable
// id is the TMDB person, the value of the actors and creators filters; absent from older servers or metadata.
data class CastMember(val name: String, val character: String = "", val photoUrl: String? = null, val id: Int? = null)

/** Catalog of a title's kind filtered on one person (category "actors" or "creators"), opened from its detail page. */
fun personCatalogQuery(itemKind: String, category: String, personId: Int): CatalogQuery {
    val kind = when (itemKind) {
        "series", "season", "episode" -> CatalogKind.SERIES
        "video" -> CatalogKind.OTHER
        else -> CatalogKind.MOVIES
    }
    val filters = if (category == "creators") CatalogFilters(creators = listOf(personId)) else CatalogFilters(actors = listOf(personId))
    return CatalogQuery(kind, filters = filters)
}

/** subtitleIndex picks a track of the file, externalSubtitleId a downloaded subtitle; at most one is set. */
data class PlaybackSelection(
    val versionId: String? = null, val audioIndex: Int? = null, val subtitleIndex: Int? = null, val externalSubtitleId: String? = null,
) {
    fun version(item: ItemDetail): Version? = item.versions.firstOrNull { it.id == versionId && it.error == null } ?: item.playableVersion

    fun normalized(item: ItemDetail): PlaybackSelection {
        val selected = version(item) ?: return PlaybackSelection()
        val audio = selected.audio.firstOrNull { it.index == audioIndex }
            ?: selected.audio.firstOrNull { it.default } ?: selected.audio.firstOrNull()
        val subtitle = subtitleIndex?.takeIf { index -> selected.subtitles.any { it.index == index } }
        val external = externalSubtitleId?.takeIf { id -> subtitle == null && selected.externalSubtitles.any { it.id == id } }
        return PlaybackSelection(selected.id, audio?.index, subtitle, external)
    }

    fun forNext(previous: ItemDetail, next: ItemDetail): PlaybackSelection {
        val current = version(previous)
        val selected = next.playableVersion ?: return PlaybackSelection()
        val audioLanguage = current?.audio?.firstOrNull { it.index == audioIndex }?.language?.takeIf { it.isNotBlank() }
        val subtitleLanguage = current?.subtitles?.firstOrNull { it.index == subtitleIndex }?.language?.takeIf { it.isNotBlank() }
        val externalLanguage = current?.externalSubtitles?.firstOrNull { it.id == externalSubtitleId }?.language
        return PlaybackSelection(selected.id,
            selected.audio.firstOrNull { audioLanguage != null && it.language == audioLanguage }?.index,
            selected.subtitles.firstOrNull { subtitleLanguage != null && it.language == subtitleLanguage }?.index,
            // A subtitle downloaded for the previous episode keeps its language when the next one has one too.
            selected.externalSubtitles.firstOrNull { externalLanguage != null && it.language == externalLanguage }?.id,
        ).normalized(next)
    }
}

@Serializable
data class ChildItem(
    val id: String,
    val kind: String,
    val title: String,
    val posterUrl: String? = null,
    val thumbUrl: String? = null,
    val seasonNumber: Int? = null,
    val episodeNumber: Int? = null,
    val watched: Boolean = false,
    val overview: String? = null,
    val runtimeMinutes: Int? = null,
    val positionSeconds: Double = 0.0,
    /** Episodes of a season the caller has not watched yet. */
    val unwatchedCount: Int? = null,
)

@Serializable
data class Ancestor(val id: String, val kind: String, val title: String, val seasonNumber: Int? = null)

@Serializable
data class PlayTarget(val id: String, val positionSeconds: Double = 0.0)

@Serializable
data class ItemDetail(
    val id: String,
    val kind: String,
    val title: String,
    val overview: String? = null,
    val year: Int? = null,
    val posterUrl: String? = null,
    val versions: List<Version>,
    val children: List<ChildItem>,
    val positionSeconds: Double = 0.0,
    val watched: Boolean = false,
    val playTarget: PlayTarget? = null,
    val next: PlayTarget? = null,
    val cast: List<CastMember> = emptyList(),
    val directors: List<String> = emptyList(),
    // Absent from older servers and empty until the title's metadata is refreshed: fall back to directors.
    val directorCredits: List<CastMember> = emptyList(),
    val seasonNumber: Int? = null,
    val episodeNumber: Int? = null,
    val originalTitle: String? = null,
    val tagline: String? = null,
    val runtimeMinutes: Int? = null,
    val rating: Double? = null,
    val genres: List<String> = emptyList(),
    val backdropUrl: String? = null,
    /** Title logo shown while paused; episodes carry their series'. Absent from older servers. */
    val logoUrl: String? = null,
    val thumbUrl: String? = null,
    /** Series then season for an episode. */
    val ancestors: List<Ancestor> = emptyList(),
    /** Set on top-level titles the user added to their watchlist; absent from servers older than the watchlist. */
    val inWatchlist: Boolean = false,
    /** Subtitles can be searched online for the versions of this item. */
    val subtitleSearch: Boolean = false,
) {
    /** Movies, series and videos can be watchlisted; seasons, episodes and parts cannot. */
    val canWatchlist: Boolean get() = ancestors.isEmpty() && kind in setOf("movie", "series", "video")
    val playableVersion: Version? get() = versions.firstOrNull { it.error == null }
    val directorPeople: List<CastMember> get() = directorCredits.ifEmpty { directors.map { CastMember(it) } }
}

@Serializable
data class ClientCaps(
    val videoCodecs: List<String>,
    val audioCodecs: List<String>,
    val hdr: Boolean = false,
    val maxHeight: Int = 1080,
    val preferredAudioCodec: String = "",
)

@Serializable
@OptIn(ExperimentalSerializationApi::class)
data class PlaybackRequest(
    val versionId: String, val start: Double, val caps: ClientCaps, val audioIndex: Int? = null, val subtitleIndex: Int? = null,
    // Left out when unset: servers older than online subtitles refuse unknown fields.
    @EncodeDefault(EncodeDefault.Mode.NEVER) val externalSubtitleId: String? = null,
)

@Serializable
data class PlaybackSession(
    val id: String, val playlistUrl: String, val start: Double, val duration: Double,
    val subtitleUrl: String? = null, val burnedSubtitle: Boolean = false,
)

@Serializable
data class ProgressRequest(val positionSeconds: Double, val durationSeconds: Double)

@Serializable
data class AutoplayRequest(val autoplayNext: Boolean)

@Serializable
data class LoginRequest(
    val username: String,
    val password: String,
    val deviceName: String = "Android TV",
    val tokenDelivery: String = "body",
)

@Serializable
data class RefreshRequest(val refreshToken: String)

@Serializable
data class SetupStatus(val needsSetup: Boolean)
