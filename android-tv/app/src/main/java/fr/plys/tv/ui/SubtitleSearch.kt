package fr.plys.tv.ui

import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.ExternalSubtitle
import java.util.Locale

/** Languages offered in the online search, as OpenSubtitles codes. */
val subtitleLanguages = listOf(
    "en", "fr", "es", "de", "it", "pt-PT", "pt-BR", "nl", "pl", "ru", "ar", "zh-CN", "zh-TW", "ja", "ko", "sv", "da",
    "no", "fi", "tr", "el", "he", "cs", "hu", "ro", "uk",
)

/** The OpenSubtitles language matching the device language, English when it is not offered. */
fun defaultSubtitleLanguage(locale: Locale = Locale.getDefault()): String {
    val regional = "${locale.language}-${locale.country}"
    return when {
        regional in subtitleLanguages -> regional
        locale.language == "pt" -> "pt-PT"
        locale.language == "zh" -> "zh-CN"
        locale.language in subtitleLanguages -> locale.language
        else -> "en"
    }
}

fun subtitleLanguageName(code: String, locale: Locale = Locale.getDefault()): String =
    Locale.forLanguageTag(code).getDisplayName(locale).replaceFirstChar { it.titlecase(locale) }.ifBlank { code }

@Composable
fun ExternalSubtitle.label(): String = listOfNotNull(
    subtitleLanguageName(language), stringResource(R.string.subtitles_hearing_impaired_short).takeIf { hearingImpaired },
    release.takeIf { it.isNotBlank() },
).joinToString(" · ")

/** Results of the online search; the first entry changes the language. */
@Composable
fun SubtitleSearchDialog(search: SubtitleSearchState, model: PlayerController) {
    var pickLanguage by remember { mutableStateOf(false) }
    if (pickLanguage) {
        ChoiceDialog(stringResource(R.string.subtitles_language), { pickLanguage = false }) {
            items(subtitleLanguages.sortedBy { subtitleLanguageName(it) }, key = { it }) { code ->
                Choice(subtitleLanguageName(code), code == search.language) {
                    pickLanguage = false
                    model.searchSubtitles(code)
                }
            }
        }
        return
    }
    ChoiceDialog(stringResource(R.string.subtitles_search_title), model::closeSubtitleSearch) {
        item {
            Choice(stringResource(R.string.subtitles_language_value, subtitleLanguageName(search.language)), false, enabled = search.downloading == null) {
                pickLanguage = true
            }
        }
        search.error?.let { item { Text(it, color = MaterialTheme.colorScheme.error) } }
        when {
            search.loading -> item { Text(stringResource(R.string.loading), color = Plys.Muted) }
            search.results.isEmpty() && search.error == null -> item { Text(stringResource(R.string.subtitles_search_empty), color = Plys.Muted) }
        }
        items(search.results, key = { it.fileId }) { result ->
            val details = listOfNotNull(
                stringResource(R.string.subtitles_hash_match).takeIf { result.hashMatch },
                stringResource(R.string.subtitles_hearing_impaired_short).takeIf { result.hearingImpaired },
                stringResource(R.string.subtitles_machine_translated).takeIf { result.machineTranslated },
                pluralStringResource(R.plurals.subtitles_downloads, result.downloadCount, result.downloadCount),
            ).joinToString(" · ")
            val label = if (search.downloading == result.fileId) stringResource(R.string.subtitles_downloading)
            else "${result.release.ifBlank { "#${result.fileId}" }} — $details"
            Choice(label, false, enabled = search.downloading == null) { model.downloadSubtitle(result) }
        }
    }
}
