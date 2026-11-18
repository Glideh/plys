package fr.plys.tv.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
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
    Box(modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color(0xFF071A2C), Color(0xFF0B2A47), Color(0xFF06182A))))) {
        Box(Modifier.fillMaxSize().background(Brush.radialGradient(
            listOf(Color(0xFF1B5A83).copy(alpha = 0.72f), Color(0xFF0B2A47).copy(alpha = 0.18f), Color.Transparent),
            radius = 1500f,
        )))
        Box(Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(250.dp).background(
            Brush.verticalGradient(listOf(Color.Transparent, Color(0xFF1474A8).copy(alpha = 0.12f))),
        ))
        Column(
            Modifier.align(Alignment.Center),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Image(painterResource(R.drawable.brand_mark), contentDescription = null, modifier = Modifier.size(184.dp))
            Text(stringResource(R.string.brand), style = MaterialTheme.typography.displayLarge, fontWeight = FontWeight.ExtraBold, color = Color(0xFFF4FAFF))
            Text(stringResource(R.string.splash_tagline), style = MaterialTheme.typography.titleLarge, color = Color(0xFFC0D8E8))
            Box(Modifier.padding(top = 8.dp).width(92.dp).height(3.dp).background(
                Brush.horizontalGradient(listOf(Color.Transparent, Color(0xFF5EB3DA), Color.Transparent)), RoundedCornerShape(2.dp),
            ))
        }
    }
}
