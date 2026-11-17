package fr.plys.tv.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.res.painterResource
import fr.plys.tv.R

/** The brand on the sign-in backdrop while the stored session is restored: no form flashes before the home screen. */
@Composable
fun SplashScreen(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxSize().background(Brush.radialGradient(listOf(Plys.Accent600.copy(alpha = 0.18f), Plys.Surface950), radius = 1400f)),
        contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(24.dp)) {
            Image(painterResource(R.drawable.brand_mark), contentDescription = null, modifier = Modifier.size(160.dp))
            Text(stringResource(R.string.brand), style = MaterialTheme.typography.displaySmall, fontWeight = FontWeight.ExtraBold, color = Plys.TextStrong)
            CircularProgressIndicator(Modifier.size(32.dp), color = Plys.Accent400, strokeWidth = 3.dp)
        }
    }
}
