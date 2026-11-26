package fr.plys.tv

import android.graphics.Bitmap
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.Alignment
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.test.platform.app.InstrumentationRegistry
import fr.plys.tv.ui.PlysTheme
import fr.plys.tv.ui.PlysButton
import fr.plys.tv.ui.Section
import fr.plys.tv.ui.SideRail
import fr.plys.tv.update.AvailableUpdate
import fr.plys.tv.update.UpdateState
import org.junit.Rule
import org.junit.Test
import org.junit.Assert.assertEquals
import java.io.File

@OptIn(ExperimentalTestApi::class)
class SideRailTest {
    @get:Rule val rule = createComposeRule()

    private val release = AvailableUpdate("99.0.0", "https://example.invalid/app.apk", 1)
    private fun text(resource: Int) =
        InstrumentationRegistry.getInstrumentation().targetContext.getString(resource)

    private fun screenshot(name: String) {
        rule.waitForIdle()
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val output = File(context.getExternalFilesDir(null), "$name.png")
        output.outputStream().use {
            rule.onRoot().captureToImage().asAndroidBitmap().compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }

    @Composable
    private fun Rail(height: Dp, update: UpdateState) {
        val content = remember { FocusRequester() }
        PlysTheme {
            Box(Modifier.size(width = 960.dp, height = height)) {
                PlysButton({}, Modifier.align(Alignment.CenterEnd).focusRequester(content).testTag("rail-test-content")) {
                    androidx.tv.material3.Text("Content")
                }
                SideRail(Section.HOME, "1.0.0", content, home = {}, search = {}, watchlist = {},
                    openKind = {}, signOut = {}, update = update)
            }
        }
    }

    @Test fun iconsStayAnchoredThroughoutExpansionAndCollapse() {
        rule.setContent { Rail(540.dp, UpdateState.Available(release)) }
        val icons = SemanticsMatcher("rail icons") {
            it.config.getOrNull(SemanticsProperties.TestTag)?.startsWith("rail-icon-") == true
        }
        fun positions() = rule.onAllNodes(icons, useUnmergedTree = true).fetchSemanticsNodes()
            .associate { it.config[SemanticsProperties.TestTag] to it.boundsInRoot }
        val folded = positions()
        assertEquals(8, folded.size)
        rule.mainClock.autoAdvance = false
        rule.onNodeWithContentDescription(text(R.string.home))
            .performSemanticsAction(SemanticsActions.RequestFocus) { it() }
        repeat(60) {
            rule.mainClock.advanceTimeByFrame()
            assertEquals("Opening frame $it", folded, positions())
        }
        rule.onNodeWithText(text(R.string.home)).assertWidthIsEqualTo(216.dp)
        rule.onNodeWithTag("rail-test-content")
            .performSemanticsAction(SemanticsActions.RequestFocus) { it() }
        repeat(60) {
            rule.mainClock.advanceTimeByFrame()
            assertEquals("Closing frame $it", folded, positions())
        }
        rule.onNodeWithContentDescription(text(R.string.home)).assertWidthIsEqualTo(40.dp)
        rule.mainClock.autoAdvance = true
    }

    @Test fun updateDoesNotShrinkSignOutOnStandardTv() {
        val update = mutableStateOf<UpdateState>(UpdateState.None)
        rule.setContent { Rail(540.dp, update.value) }
        val signOut = text(R.string.sign_out)
        rule.onNodeWithContentDescription(signOut).assertIsDisplayed().assertHeightIsEqualTo(40.dp)
            .assertWidthIsEqualTo(40.dp)
        rule.runOnIdle { update.value = UpdateState.Available(release) }
        rule.onNodeWithContentDescription(signOut).assertIsDisplayed().assertHeightIsEqualTo(40.dp)
        screenshot("menu-update-closed")
        rule.onNodeWithContentDescription(text(R.string.home))
            .performSemanticsAction(SemanticsActions.RequestFocus) { it() }
        rule.waitForIdle()
        rule.onNodeWithText(signOut).assertIsDisplayed().assertHeightIsEqualTo(40.dp)
            .assertWidthIsEqualTo(216.dp)
        screenshot("menu-update-open")
        rule.onNodeWithText(signOut).performSemanticsAction(SemanticsActions.RequestFocus) { it() }
        rule.onNodeWithText(signOut).assertIsFocused()
    }

    @Test fun remoteCanReachFullSizeSignOutBelowUpdateInstructionsOnShortTv() {
        rule.setContent { Rail(360.dp, UpdateState.NeedsPermission(release)) }
        rule.onNodeWithContentDescription(text(R.string.home))
            .performSemanticsAction(SemanticsActions.RequestFocus) { it() }
        rule.waitForIdle()
        repeat(7) {
            rule.onNode(isFocused()).performKeyInput { pressKey(Key.DirectionDown) }
        }
        rule.onNodeWithText(text(R.string.sign_out)).assertIsFocused().assertIsDisplayed()
            .assertHeightIsEqualTo(40.dp)
    }
}
