package fr.plys.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import fr.plys.tv.ui.PlysTheme
import fr.plys.tv.ui.CatalogViewModel
import fr.plys.tv.ui.CatalogApp
import fr.plys.tv.ui.PlaybackScreen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        val splashScreen = installSplashScreen()
        super.onCreate(savedInstanceState)
        splashScreen.setOnExitAnimationListener { splash ->
            splash.view.animate().alpha(0f).setDuration(280L).withEndAction { splash.remove() }.start()
        }
        setContent {
            PlysTheme {
                    val model: CatalogViewModel = viewModel()
                    val state by model.state.collectAsStateWithLifecycle()
                    val item = state.playback
                    if (item != null) {
                        PlaybackScreen(item, state.playbackPosition, model.api, model::closePlayer, state.playbackSource, state.playbackSelection)
                    } else {
                        CatalogApp(model, state, Modifier.fillMaxSize())
                    }
            }
        }
    }
}
