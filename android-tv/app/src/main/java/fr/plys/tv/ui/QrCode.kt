package fr.plys.tv.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.qrcode.QRCodeWriter
import com.google.zxing.qrcode.decoder.ErrorCorrectionLevel

/** The dark modules of a QR code of [text], one row per line, without the quiet zone. */
fun qrModules(text: String): List<BooleanArray> {
    // A requested size of zero renders one pixel per module.
    val matrix = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, 0, 0,
        mapOf(EncodeHintType.MARGIN to 0, EncodeHintType.ERROR_CORRECTION to ErrorCorrectionLevel.M))
    return List(matrix.height) { y -> BooleanArray(matrix.width) { x -> matrix[x, y] } }
}

/** Black on white whatever the theme, with the light margin scanners expect around the symbol. */
@Composable
fun QrCode(text: String, description: String, modifier: Modifier = Modifier) {
    val modules = remember(text) { qrModules(text) }
    Canvas(modifier.background(Color.White, Plys.Small).padding(16.dp).semantics { contentDescription = description }) {
        val cell = size.minDimension / modules.size
        modules.forEachIndexed { y, row ->
            row.forEachIndexed { x, dark ->
                // Slightly wider cells leave no hairline between neighbours once scaled.
                if (dark) drawRect(Color.Black, Offset(x * cell, y * cell), Size(cell + 0.5f, cell + 0.5f))
            }
        }
    }
}
