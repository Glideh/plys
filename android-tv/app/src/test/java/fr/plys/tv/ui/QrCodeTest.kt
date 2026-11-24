package fr.plys.tv.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class QrCodeTest {
    @Test fun `modules form a square symbol with its finder patterns`() {
        val modules = qrModules("https://plys.example/link?code=BCDF-GHJK")
        val size = modules.size
        assertTrue((size - 21) % 4 == 0)
        modules.forEach { assertEquals(size, it.size) }
        // The three finder patterns start with a dark corner and a light ring inside it.
        assertTrue(modules[0][0] && modules[0][size - 1] && modules[size - 1][0])
        assertFalse(modules[1][1])
        assertTrue(modules[2][2])
    }
}
