package fr.plys.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.darkColorScheme
import fr.plys.tv.ui.CatalogViewModel
import fr.plys.tv.ui.CatalogApp
import fr.plys.tv.ui.PlaybackScreen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            androidx.compose.material3.MaterialTheme(
                colorScheme = androidx.compose.material3.darkColorScheme(),
            ) {
                MaterialTheme(colorScheme = darkColorScheme(
                    background = Color(0xFF111827),
                    surface = Color(0xFF1F2937),
                    primary = Color(0xFFA5B4FC),
                )) {
                    val model: CatalogViewModel = viewModel()
                    val state by model.state.collectAsStateWithLifecycle()
                    val item = state.playback
                    if (item != null) {
                        PlaybackScreen(item, state.playbackPosition, model.api, model::closePlayer)
                    } else {
                        CatalogApp(model, state, Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background))
                    }
                }
            }
        }
    }
}
