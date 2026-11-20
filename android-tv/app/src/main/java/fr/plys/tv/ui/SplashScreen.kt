package fr.plys.tv.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
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
    BoxWithConstraints(modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color(0xFF071A2C), Color(0xFF0B2A47), Color(0xFF06182A))))) {
        val compact = maxHeight < 500.dp
        val logoSize = if (compact) (maxHeight * 0.38f).coerceAtMost(184.dp) else 184.dp
        val titleStyle = if (compact) MaterialTheme.typography.headlineLarge else MaterialTheme.typography.displayLarge
        val taglineStyle = if (compact) MaterialTheme.typography.titleMedium else MaterialTheme.typography.titleLarge
        val logoTipOffset = logoSize * (82f / 184f)
        val textSpacing = if (compact) 8.dp else 12.dp
        val textTopGap = if (compact) 12.dp else 16.dp
        Box(Modifier.fillMaxSize().background(Brush.radialGradient(
            listOf(Color(0xFF1B5A83).copy(alpha = 0.72f), Color(0xFF0B2A47).copy(alpha = 0.18f), Color.Transparent),
            radius = 1500f,
        )))
        Box(Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(250.dp).background(
            Brush.verticalGradient(listOf(Color.Transparent, Color(0xFF1474A8).copy(alpha = 0.12f))),
        ))
        Box(
            Modifier.align(Alignment.Center).offset(y = logoTipOffset).width(92.dp).height(3.dp)
                .background(Brush.horizontalGradient(listOf(Color.Transparent, Color(0xFF5EB3DA), Color.Transparent)), RoundedCornerShape(2.dp))
                .testTag("splashGlow"),
        )
        Image(
            painterResource(R.drawable.brand_mark), contentDescription = null,
            modifier = Modifier.align(Alignment.Center).size(logoSize).testTag("splashLogo"),
        )
        Column(
            Modifier.align(Alignment.TopCenter).padding(top = maxHeight / 2 + logoSize / 2 + textTopGap),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(textSpacing),
        ) {
            Text(stringResource(R.string.brand), style = titleStyle, fontWeight = FontWeight.ExtraBold, color = Color(0xFFF4FAFF))
            Text(stringResource(R.string.splash_tagline), style = taglineStyle, color = Color(0xFFC0D8E8))
        }
    }
}
