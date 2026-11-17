package fr.plys.tv.ui

import android.view.KeyEvent
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.gestures.animateScrollBy
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.window.Dialog
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Audiotrack
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.HighQuality
import androidx.compose.material.icons.outlined.Subtitles
import androidx.compose.material3.Icon
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import kotlinx.coroutines.launch
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.ui.semantics.Role
import fr.plys.tv.data.CastMember
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.MediaTrack
import fr.plys.tv.data.PlaybackSelection
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.ServerRef
import fr.plys.tv.data.VersionChoice
import java.util.Locale

fun MediaTrack.trackLabel(): String = listOfNotNull(
    language.takeIf { it.isNotBlank() }, title.takeIf { it.isNotBlank() },
    "#${index + 1}".takeIf { language.isBlank() && title.isBlank() },
    codec.uppercase().takeIf { it.isNotBlank() }, channelLayout(channels),
).joinToString(" · ")

/**
 * The track as named on the detail screen's button, which has little room: its short language code and title,
 * without the formats that the version summary and the dialog already show. Falls back to the full label when both
 * are blank.
 */
fun MediaTrack.shortLabel(): String =
    listOfNotNull(shortLanguage(language), title.takeIf { it.isNotBlank() }).joinToString(" · ").ifEmpty { trackLabel() }

/** ISO 639-2 codes (bibliographic, as ffprobe reports them, and terminology) to their ISO 639-1 code. */
private val iso6392to1: Map<String, String> by lazy {
    val terminology = Locale.getISOLanguages().mapNotNull { code ->
        runCatching { Locale.forLanguageTag(code).isO3Language }.getOrNull()?.takeIf { it.length == 3 }?.let { it to code }
    }.toMap()
    terminology + mapOf(
        "fre" to "fr", "ger" to "de", "chi" to "zh", "dut" to "nl", "gre" to "el", "cze" to "cs", "per" to "fa",
        "rum" to "ro", "slo" to "sk", "alb" to "sq", "arm" to "hy", "baq" to "eu", "bur" to "my", "geo" to "ka",
        "ice" to "is", "mac" to "mk", "mao" to "mi", "may" to "ms", "tib" to "bo", "wel" to "cy", "heb" to "he",
    )
}

/**
 * A track language as a short code: the ISO 639-1 language in upper case, followed by the region in lower case when
 * there is one ("fre" → "FR", "eng" → "EN", "fr-CA" → "FR-ca"). Codes without a two-letter equivalent are kept,
 * in upper case. Null when the language is blank or undetermined ("und").
 */
fun shortLanguage(code: String): String? {
    val parts = code.trim().split('-', '_').filter { it.isNotBlank() }
    val base = parts.firstOrNull()?.lowercase(Locale.ROOT)?.takeIf { it != "und" } ?: return null
    val language = (if (base.length == 3) iso6392to1[base] ?: base else base).uppercase(Locale.ROOT)
    val region = parts.getOrNull(1)?.lowercase(Locale.ROOT)
    return if (region == null) language else "$language-$region"
}

fun channelLayout(channels: Int): String? = when (channels) {
    0 -> null
    1 -> "Mono"
    2 -> "2.0"
    6 -> "5.1"
    8 -> "7.1"
    else -> "$channels ch"
}

/** Resolution and formats of the version that will play, e.g. "1080p · HDR · HEVC · EAC3 5.1". */
fun versionSummary(item: ItemDetail, selection: PlaybackSelection): String {
    val chosen = selection.normalized(item)
    val version = chosen.version(item) ?: return ""
    val audio = version.audio.firstOrNull { it.index == chosen.audioIndex }
    return listOfNotNull(
        version.resolution.takeIf { it.isNotBlank() }, "HDR".takeIf { version.hdr },
        version.videoCodec.uppercase().takeIf { it.isNotBlank() },
        audio?.let { listOfNotNull(it.codec.uppercase().takeIf { codec -> codec.isNotBlank() }, channelLayout(it.channels)).joinToString(" ") }?.takeIf { it.isNotBlank() },
    ).joinToString(" · ")
}

@Composable
fun VersionSummary(item: ItemDetail, selection: PlaybackSelection) {
    val summary = versionSummary(item, selection)
    if (summary.isNotEmpty()) Text(summary, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
}

/** The overview, shortened; selecting it shows the whole text. */
@Composable
fun Synopsis(text: String, open: () -> Unit) {
    val palette = MaterialTheme.colorScheme
    val shape = RoundedCornerShape(6.dp)
    Surface(
        // Shifted by its padding so the text lines up with the title while the focus frame surrounds it.
        onClick = open, modifier = Modifier.fillMaxWidth().offset(x = (-8).dp),
        shape = ClickableSurfaceDefaults.shape(shape),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border = ClickableSurfaceDefaults.border(focusedBorder = Border(BorderStroke(2.dp, palette.primary), shape = shape)),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = androidx.compose.ui.graphics.Color.Transparent, contentColor = palette.onSurface,
            focusedContainerColor = palette.onSurface.copy(alpha = 0.08f), focusedContentColor = palette.onSurface,
        ),
    ) {
        Text(text, modifier = Modifier.padding(8.dp), maxLines = 3, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
fun SynopsisDialog(title: String, text: String, dismiss: () -> Unit) {
    val scroll = rememberScrollState()
    val scope = rememberCoroutineScope()
    var focused by remember { mutableStateOf(false) }
    Dialog(onDismissRequest = dismiss) {
        androidx.tv.material3.Surface(colors = androidx.tv.material3.SurfaceDefaults.colors(containerColor = Plys.Surface900), shape = Plys.Large) {
            Column(Modifier.widthIn(max = 760.dp).fillMaxWidth().heightIn(max = 460.dp).padding(24.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(title, style = MaterialTheme.typography.headlineSmall, color = Plys.TextStrong)
                // The text is focusable so the remote scrolls long overviews.
                Text(text, style = MaterialTheme.typography.bodyLarge,
                    modifier = Modifier.weight(1f, fill = false)
                        .border(2.dp, if (focused) MaterialTheme.colorScheme.primary else androidx.compose.ui.graphics.Color.Transparent, RoundedCornerShape(6.dp))
                        .focusInitially()
                        .onFocusChanged { focused = it.isFocused }
                        .onKeyEvent { event ->
                            val step = when (event.nativeKeyEvent.keyCode) { KeyEvent.KEYCODE_DPAD_DOWN -> 160f; KeyEvent.KEYCODE_DPAD_UP -> -160f; else -> 0f }
                            if (step != 0f && event.nativeKeyEvent.action == KeyEvent.ACTION_DOWN && scroll.canScroll(step)) {
                                scope.launch { scroll.animateScrollBy(step) }; true
                            } else false
                        }
                        .focusable().verticalScroll(scroll).padding(8.dp))
                Action(R.string.back, onClick = dismiss)
            }
        }
    }
}

private fun androidx.compose.foundation.ScrollState.canScroll(step: Float) = if (step > 0) value < maxValue else value > 0

/** "Home · 2160p · HDR · HEVC · 18.4 GB": the server is named only when the title is on several. */
@Composable
fun versionChoiceLabel(choice: VersionChoice, withServer: Boolean): String {
    val context = androidx.compose.ui.platform.LocalContext.current
    return listOfNotNull(
        choice.copy.source?.serverName?.takeIf { withServer && it.isNotBlank() },
        choice.version.label.takeIf { it.isNotBlank() },
        choice.version.size.takeIf { it > 0 }?.let { android.text.format.Formatter.formatFileSize(context, it) },
    ).joinToString(" · ")
}

/**
 * The version, audio and subtitle pickers of the detail screen. Versions gather every server holding the title,
 * like the web's item page; a series or season, which has no version of its own, picks its server instead.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun PlaybackSelectionControls(screen: Screen.Detail, model: CatalogViewModel) {
    val item = screen.playable
    val chosen = screen.selection.normalized(item)
    val version = chosen.version(item)
    val choices = screen.versionChoices
    val servers = screen.serverChoices
    val withServer = choices.distinctBy { it.copy.source?.serverId }.size > 1
    val audio = version?.audio.orEmpty()
    val subtitles = version?.subtitles.orEmpty()
    if (choices.size <= 1 && servers.isEmpty() && audio.size <= 1 && subtitles.isEmpty()) return
    var menu by remember { mutableStateOf("") }
    val current = choices.firstOrNull { it.copy.source?.serverId == screen.source?.serverId && it.copy.item.id == item.id && it.version.id == version?.id }
    val subtitle = subtitles.firstOrNull { it.index == chosen.subtitleIndex }
    // Wraps onto several lines rather than scrolling, so every choice stays visible and the focused button,
    // slightly enlarged, is never cut off. The icon names the picker, the button only shows the current choice.
    FlowRow(Modifier.fillMaxWidth().padding(FocusGutter), horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)) {
        if (servers.isNotEmpty()) TrackButton(Icons.Outlined.Dns, stringResource(R.string.server_name), screen.source?.serverName.orEmpty()) { menu = "server" }
        if (choices.size > 1) {
            val label = current?.let { versionChoiceLabel(it, withServer) } ?: version?.label.orEmpty()
            TrackButton(Icons.Outlined.HighQuality, stringResource(R.string.version), label) { menu = "version" }
        }
        if (audio.size > 1) TrackButton(Icons.Outlined.Audiotrack, stringResource(R.string.audio),
            audio.firstOrNull { it.index == chosen.audioIndex }?.shortLabel().orEmpty()) { menu = "audio" }
        if (subtitles.isNotEmpty()) TrackButton(Icons.Outlined.Subtitles, stringResource(R.string.subtitles),
            subtitle?.shortLabel() ?: stringResource(R.string.no_subtitles)) { menu = "subtitles" }
    }
    when (menu) {
        "server" -> ChoiceDialog(stringResource(R.string.server_name), { menu = "" }) {
            items(servers, key = { "${it.serverId}/${it.itemId}" }) { server ->
                Choice(server.serverName, server.serverId == screen.source?.serverId) { model.selectSource(server); menu = "" }
            }
        }
        "version" -> ChoiceDialog(stringResource(R.string.version), { menu = "" }) {
            items(choices, key = { "${it.copy.source?.serverId}/${it.version.id}" }) { candidate ->
                Choice(versionChoiceLabel(candidate, withServer), candidate == current, enabled = candidate.version.error == null) {
                    model.selectVersion(candidate); menu = ""
                }
            }
        }
        "audio" -> ChoiceDialog(stringResource(R.string.audio), { menu = "" }) {
            items(audio, key = { it.index }) { track ->
                Choice(track.trackLabel(), track.index == chosen.audioIndex) { model.selectPlayback(chosen.copy(audioIndex = track.index)); menu = "" }
            }
        }
        "subtitles" -> ChoiceDialog(stringResource(R.string.subtitles), { menu = "" }) {
            item { Choice(stringResource(R.string.no_subtitles), chosen.subtitleIndex == null) { model.selectPlayback(chosen.copy(subtitleIndex = null)); menu = "" } }
            items(subtitles, key = { it.index }) { track ->
                Choice(track.trackLabel(), track.index == chosen.subtitleIndex) { model.selectPlayback(chosen.copy(subtitleIndex = track.index)); menu = "" }
            }
        }
    }
}

@Composable
private fun TrackButton(icon: androidx.compose.ui.graphics.vector.ImageVector, name: String, label: String, onClick: () -> Unit) {
    PlysButton(onClick = onClick, modifier = Modifier.widthIn(max = 340.dp),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 14.dp, vertical = 6.dp)) {
        Icon(icon, contentDescription = name, modifier = Modifier.size(18.dp), tint = androidx.tv.material3.LocalContentColor.current)
        androidx.compose.foundation.layout.Spacer(Modifier.width(8.dp))
        Text(label.ifEmpty { name }, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.bodySmall)
    }
}

/** One entry of a choice dialog; the current one is checked and takes the focus when the dialog opens. */
@Composable
fun Choice(label: String, selected: Boolean, enabled: Boolean = true, onClick: () -> Unit) {
    PlysButton(onClick = onClick, enabled = enabled, modifier = Modifier.fillMaxWidth().focusInitially(selected)) {
        Box(Modifier.size(22.dp)) {
            if (selected) Icon(Icons.Outlined.Check, contentDescription = null, tint = androidx.tv.material3.LocalContentColor.current, modifier = Modifier.fillMaxSize())
        }
        androidx.compose.foundation.layout.Spacer(Modifier.width(12.dp))
        Text(label, modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** Cast, directors or creators, each in a disc with the name below on one line; a person with a TMDB ID opens their titles. */
@Composable
fun PeopleRow(title: Int, people: List<CastMember>, api: PlysApi, source: ServerRef?, open: (CastMember) -> Unit) {
    Column {
        RailTitle(title)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp), contentPadding = RowFocusPadding) {
            items(people) { person ->
                var focused by remember { mutableStateOf(false) }
                val interactions = remember { MutableInteractionSource() }
                val action = Modifier.width(128.dp).onFocusChanged { focused = it.hasFocus }
                Column(if (person.id != null) action.clickable(interactionSource = interactions, indication = null, role = Role.Button) { open(person) }
                    else action.focusable(),
                    horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Box(Modifier.size(80.dp).clip(CircleShape)
                        .border(if (focused) 3.dp else 1.dp, if (focused) Plys.Accent300 else Plys.Hairline, CircleShape)) {
                        // Pictures are portraits: crop them to fill the disc.
                        if (person.photoUrl != null) coil.compose.AsyncImage(model = api.posterUrl(person.photoUrl, source), contentDescription = null,
                            modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.surfaceVariant), contentScale = androidx.compose.ui.layout.ContentScale.Crop)
                        else Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.surfaceVariant)) {
                            Text(person.name.take(1), modifier = Modifier.align(Alignment.Center), style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                    Text(person.name, style = MaterialTheme.typography.bodyMedium, textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                        maxLines = 1, overflow = TextOverflow.Ellipsis, color = MaterialTheme.colorScheme.onSurface)
                    if (person.character.isNotBlank()) Text(person.character, style = MaterialTheme.typography.bodySmall, textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                        maxLines = 1, overflow = TextOverflow.Ellipsis, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
    }
}
