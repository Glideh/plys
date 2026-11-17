package fr.plys.tv.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Logout
import androidx.compose.material.icons.outlined.FavoriteBorder
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import fr.plys.tv.R
import fr.plys.tv.data.CatalogKind

/** Width of the folded rail: the screens start after it. */
val RailWidth = 64.dp
private val ExpandedRailWidth = 260.dp

enum class Section { HOME, SEARCH, WATCHLIST, MOVIES, SERIES, OTHER, NONE }

/** The rail entry of a screen. */
fun sectionOf(screen: Screen): Section = when (screen) {
    is Screen.Home -> Section.HOME
    Screen.Search -> Section.SEARCH
    is Screen.Watchlist -> Section.WATCHLIST
    is Screen.Catalog -> sectionOf(screen.query.kind)
    else -> Section.NONE
}

fun sectionOf(kind: CatalogKind) = when (kind) {
    CatalogKind.MOVIES -> Section.MOVIES
    CatalogKind.SERIES -> Section.SERIES
    CatalogKind.OTHER -> Section.OTHER
}

/**
 * Navigation always one press away, the Google TV pattern: a column of icons on the left that unfolds over the
 * screen, with labels, while it holds the focus. Pressing left from the first column of any screen reaches it, and
 * entering it lands on the current section rather than on whatever item is level with the focus. Right goes back to
 * the element of the screen that held the focus before.
 */
@OptIn(androidx.compose.ui.ExperimentalComposeUiApi::class)
@Composable
fun SideRail(
    current: Section, serverVersion: String, content: FocusRequester, modifier: Modifier = Modifier,
    home: () -> Unit, search: () -> Unit, watchlist: () -> Unit, openKind: (CatalogKind) -> Unit, signOut: () -> Unit,
) {
    var expanded by remember { mutableStateOf(false) }
    val width by animateDpAsState(if (expanded) ExpandedRailWidth else RailWidth, label = "railWidth")
    val requesters = remember { Section.entries.associateWith { FocusRequester() } }
    val scrim = if (expanded) Brush.horizontalGradient(0f to Plys.Surface950, 0.7f to Plys.Surface950.copy(alpha = 0.97f), 1f to Plys.Surface950.copy(alpha = 0f))
        else Brush.horizontalGradient(listOf(Plys.Surface950, Plys.Surface950))
    Box(modifier.fillMaxHeight().width(if (expanded) width + 120.dp else width).background(scrim)) {
        Column(
            Modifier.fillMaxHeight().width(width).padding(vertical = 24.dp, horizontal = 8.dp)
                .onFocusChanged { expanded = it.hasFocus }
                .focusProperties {
                    @Suppress("DEPRECATION")
                    enter = { requesters[current.takeIf { it != Section.NONE } ?: Section.HOME] ?: FocusRequester.Default }
                },
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Brand(expanded, Modifier.padding(start = 4.dp, bottom = 24.dp))
            RailItem(Icons.Outlined.Home, R.string.home, current == Section.HOME, expanded, requesters.getValue(Section.HOME), content, home)
            RailItem(Icons.Outlined.Search, R.string.search, current == Section.SEARCH, expanded, requesters.getValue(Section.SEARCH), content, search)
            RailItem(Icons.Outlined.FavoriteBorder, R.string.watchlist, current == Section.WATCHLIST, expanded, requesters.getValue(Section.WATCHLIST), content, watchlist)
            CatalogKind.entries.forEach { kind ->
                val section = sectionOf(kind)
                RailItem(kindIcon(kind), kindLabel(kind), current == section, expanded, requesters.getValue(section), content) { openKind(kind) }
            }
            Spacer(Modifier.weight(1f))
            RailItem(Icons.AutoMirrored.Outlined.Logout, R.string.sign_out, false, expanded, requesters.getValue(Section.NONE), content, signOut)
            // The web's footer: the server version, readable once the rail unfolds.
            if (expanded && serverVersion.isNotEmpty()) {
                Text(stringResource(R.string.server_version, serverVersion), style = MaterialTheme.typography.labelSmall,
                    color = Plys.Subtle, maxLines = 1, softWrap = false, modifier = Modifier.padding(start = 12.dp))
            }
        }
    }
}

/** The brand mark and name, matching the web header. */
@Composable
fun Brand(showName: Boolean = true, modifier: Modifier = Modifier) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Image(painterResource(R.drawable.brand_mark), contentDescription = if (showName) null else stringResource(R.string.brand), modifier = Modifier.size(40.dp))
        AnimatedVisibility(showName, enter = fadeIn(), exit = fadeOut()) {
            // Never wrapped: while the rail unfolds or folds the name has almost no room, and wrapping it letter by
            // letter would make this row taller and push every rail entry down during the animation.
            Text(stringResource(R.string.brand), style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.ExtraBold,
                color = Plys.TextStrong, maxLines = 1, softWrap = false)
        }
    }
}

@Composable
private fun RailItem(icon: ImageVector, label: Int, selected: Boolean, expanded: Boolean, requester: FocusRequester, content: FocusRequester, onClick: () -> Unit) {
    val text = stringResource(label)
    PlysButton(
        onClick = onClick, shape = RoundedCornerShape(12.dp),
        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 12.dp),
        // Right always leaves the rail for the screen, whatever the unfolded rail happens to cover.
        modifier = Modifier.fillMaxWidth().focusRequester(requester).focusProperties { right = content },
    ) {
        // Resting content is light and focused content dark: the current section keeps an accent that reads on both.
        val focused = LocalContentColor.current != Plys.Text
        Icon(icon, contentDescription = text.takeIf { !expanded }, modifier = Modifier.size(24.dp),
            tint = if (!selected) LocalContentColor.current else if (focused) Plys.Accent600 else Plys.Accent400)
        if (expanded) {
            Spacer(Modifier.width(16.dp))
            Text(text, style = MaterialTheme.typography.titleMedium, fontWeight = if (selected) FontWeight.Bold else FontWeight.SemiBold,
                maxLines = 1, softWrap = false, modifier = Modifier.weight(1f))
        }
    }
}
