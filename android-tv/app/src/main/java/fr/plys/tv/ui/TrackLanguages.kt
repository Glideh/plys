package fr.plys.tv.ui

import android.content.Context
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.MediaTrack
import fr.plys.tv.data.PlaybackSelection
import java.util.Locale

/**
 * A language code reduced to what two tracks in the same language share: the ISO 639-1 code when known, without the
 * region ("fre", "fra", "fr" and "fr-CA" all give "fr"). Null when the language is blank or undetermined.
 */
fun languageKey(code: String?): String? = code?.let(::shortLanguage)?.substringBefore('-')?.lowercase(Locale.ROOT)

/**
 * Audio and subtitle languages last picked on a detail page or in the player, kept on this device, as the web client
 * keeps them in the browser. They preset the tracks of the next title played; a null subtitle means subtitles off.
 */
data class TrackLanguages(val audio: String? = null, val subtitle: String? = null) {
    /** A track without a language keeps the previous preference. */
    fun withAudio(language: String): TrackLanguages = languageKey(language)?.let { copy(audio = it) } ?: this

    /** Null turns subtitles off; a subtitle without a language keeps the previous preference. */
    fun withSubtitle(language: String?): TrackLanguages =
        if (language == null) copy(subtitle = null) else languageKey(language)?.let { copy(subtitle = it) } ?: this

    /** The tracks a title opens with: the first ones in these languages, otherwise the default audio and no subtitles. */
    fun selection(item: ItemDetail, versionId: String? = null): PlaybackSelection {
        val base = PlaybackSelection(versionId).normalized(item)
        val version = base.version(item) ?: return base
        return PlaybackSelection(
            version.id,
            preferredTrack(version.audio, audio)?.index ?: base.audioIndex,
            preferredTrack(version.subtitles, subtitle)?.index,
        ).normalized(item)
    }

    /** The languages once the user switched from [previous] to [next]; only the tracks that changed count. */
    fun after(item: ItemDetail, previous: PlaybackSelection, next: PlaybackSelection): TrackLanguages {
        val version = next.version(item) ?: return this
        var result = this
        if (next.audioIndex != previous.audioIndex) {
            version.audio.firstOrNull { it.index == next.audioIndex }?.let { result = result.withAudio(it.language) }
        }
        if (next.subtitleIndex != previous.subtitleIndex || next.externalSubtitleId != previous.externalSubtitleId) {
            val language = when {
                next.subtitleIndex != null -> version.subtitles.firstOrNull { it.index == next.subtitleIndex }?.language ?: return result
                next.externalSubtitleId != null -> version.externalSubtitles.firstOrNull { it.id == next.externalSubtitleId }?.language ?: return result
                else -> null
            }
            result = result.withSubtitle(language)
        }
        return result
    }
}

/**
 * The first track in [language], or null. A subtitle that is not forced comes first: forced ones only cover the lines
 * spoken in another language.
 */
fun preferredTrack(tracks: List<MediaTrack>, language: String?): MediaTrack? {
    if (language == null) return null
    val matching = tracks.filter { languageKey(it.language) == language }
    return matching.firstOrNull { !it.forced } ?: matching.firstOrNull()
}

/** The languages picked on this device, in its local preferences. */
class TrackLanguageStore(context: Context) {
    private val preferences = context.getSharedPreferences("player", Context.MODE_PRIVATE)

    fun read(): TrackLanguages = TrackLanguages(
        languageKey(preferences.getString(AUDIO, null)), languageKey(preferences.getString(SUBTITLE, null)),
    )

    fun save(languages: TrackLanguages) {
        preferences.edit().putString(AUDIO, languages.audio).putString(SUBTITLE, languages.subtitle).apply()
    }

    /** Remembers the languages of the tracks the user switched to. */
    fun remember(item: ItemDetail, previous: PlaybackSelection, next: PlaybackSelection) {
        val current = read()
        val updated = current.after(item, previous, next)
        if (updated != current) save(updated)
    }

    private companion object {
        const val AUDIO = "audio_language"
        const val SUBTITLE = "subtitle_language"
    }
}
