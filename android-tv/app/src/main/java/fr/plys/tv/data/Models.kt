package fr.plys.tv.data

import kotlinx.serialization.Serializable

@Serializable
data class User(val id: String, val username: String, val locale: String)

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
)

@Serializable
data class ItemPage(val items: List<ItemCard>, val total: Int, val offset: Int, val limit: Int)

@Serializable
data class Version(val id: String, val label: String, val durationSeconds: Double? = null, val error: String? = null)

@Serializable
data class ChildItem(
    val id: String,
    val kind: String,
    val title: String,
    val posterUrl: String? = null,
    val thumbUrl: String? = null,
    val seasonNumber: Int? = null,
    val episodeNumber: Int? = null,
    val partNumber: Int? = null,
    val watched: Boolean = false,
)

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
) {
    val playableVersion: Version? get() = versions.firstOrNull { it.error == null }
}

@Serializable
data class ClientCaps(
    val videoCodecs: List<String>,
    val audioCodecs: List<String>,
    val hdr: Boolean = false,
    val maxHeight: Int = 1080,
)

@Serializable
data class PlaybackRequest(val versionId: String, val start: Double, val caps: ClientCaps)

@Serializable
data class PlaybackSession(val id: String, val playlistUrl: String, val start: Double, val duration: Double)

@Serializable
data class ProgressRequest(val positionSeconds: Double, val durationSeconds: Double)

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
