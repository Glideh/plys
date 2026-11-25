package fr.plys.tv.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.Canvas
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
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
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
        Box(Modifier.align(Alignment.TopCenter).fillMaxWidth().height(250.dp).background(
            Brush.verticalGradient(listOf(Color(0xFF1474A8).copy(alpha = 0.12f), Color.Transparent)),
        ))
        Image(
            painterResource(R.drawable.splash_wave_glow),
            contentDescription = null,
            contentScale = ContentScale.FillBounds,
            modifier = Modifier.align(Alignment.TopCenter).fillMaxWidth().height(maxHeight * 0.32f)
                .graphicsLayer {
                    scaleY = -1f
                    compositingStrategy = CompositingStrategy.Offscreen
                }
                .drawWithContent {
                    drawContent()
                    drawRect(
                        brush = Brush.horizontalGradient(
                            listOf(Color.White.copy(alpha = 0.08f), Color.White, Color.White.copy(alpha = 0.08f)),
                        ),
                        blendMode = BlendMode.DstIn,
                    )
                },
        )
        Canvas(Modifier.fillMaxSize().graphicsLayer { scaleY = -1f }) {
            val w = size.width
            val h = size.height
            drawRect(
                Brush.radialGradient(
                    listOf(Color(0xFF159FE6).copy(alpha = 0.38f), Color(0xFF0B578E).copy(alpha = 0.14f), Color.Transparent),
                    center = Offset(w * 0.52f, h * 0.98f),
                    radius = w * 0.38f,
                ),
            )
            val wave = Path().apply {
                moveTo(0f, h * 0.88f)
                cubicTo(w * 0.22f, h * 0.90f, w * 0.35f, h * 0.98f, w * 0.62f, h * 0.98f)
                cubicTo(w * 0.78f, h * 0.98f, w * 0.89f, h * 0.95f, w, h * 0.94f)
            }
            val waveFill = Path().apply {
                addPath(wave)
                lineTo(w, h)
                lineTo(0f, h)
                close()
            }
            drawPath(
                waveFill,
                Brush.verticalGradient(
                    listOf(Color(0xFF0C4F80).copy(alpha = 0.28f), Color(0xFF031426).copy(alpha = 0.38f)),
                    startY = h * 0.88f,
                    endY = h,
                ),
            )
            drawPath(
                wave,
                Brush.horizontalGradient(
                    listOf(Color(0xFF37A9E8).copy(alpha = 0.08f), Color(0xFF70D2FF).copy(alpha = 0.8f), Color(0xFF1F8CCB).copy(alpha = 0.12f)),
                    startX = 0f,
                    endX = w,
                ),
                style = Stroke(width = 1.5.dp.toPx()),
            )
        }
        Box(
            Modifier.align(Alignment.Center).offset(y = logoTipOffset).size(180.dp)
                .graphicsLayer { scaleY = 0.16f }
                .background(Brush.radialGradient(listOf(Color(0xFF62C9FA).copy(alpha = 0.7f), Color(0xFF1787C3).copy(alpha = 0.25f), Color.Transparent))),
        )
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
