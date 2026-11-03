package com.amitia.amitia_app.virtualdisplay.preview

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class VirtualDisplayPreviewGeometryTest {
    @Test
    fun detectsAllCornerHitAreas() {
        assertEquals(
            VirtualDisplayResizeCorner.TOP_START,
            VirtualDisplayPreviewGeometry.detectCorner(10f, 10f, 400, 700, 30f),
        )
        assertEquals(
            VirtualDisplayResizeCorner.TOP_END,
            VirtualDisplayPreviewGeometry.detectCorner(390f, 10f, 400, 700, 30f),
        )
        assertEquals(
            VirtualDisplayResizeCorner.BOTTOM_START,
            VirtualDisplayPreviewGeometry.detectCorner(10f, 690f, 400, 700, 30f),
        )
        assertEquals(
            VirtualDisplayResizeCorner.BOTTOM_END,
            VirtualDisplayPreviewGeometry.detectCorner(390f, 690f, 400, 700, 30f),
        )
        assertNull(
            VirtualDisplayPreviewGeometry.detectCorner(200f, 300f, 400, 700, 30f),
        )
    }

    @Test
    fun resizesFromBottomEndWhileKeepingTopStartAnchored() {
        val frame = VirtualDisplayPreviewGeometry.resize(
            start = VirtualDisplayWindowFrame(100, 200, 400, 800),
            corner = VirtualDisplayResizeCorner.BOTTOM_END,
            deltaX = 100f,
            deltaY = 0f,
            aspectRatio = 2f,
            minWidth = 120,
            maxWidth = 1000,
            screenWidth = 1220,
            screenHeight = 2712,
        )
        assertEquals(VirtualDisplayWindowFrame(100, 200, 500, 1000), frame)
    }

    @Test
    fun resizesFromTopStartWhileKeepingBottomEndAnchored() {
        val frame = VirtualDisplayPreviewGeometry.resize(
            start = VirtualDisplayWindowFrame(100, 200, 400, 800),
            corner = VirtualDisplayResizeCorner.TOP_START,
            deltaX = -100f,
            deltaY = 0f,
            aspectRatio = 2f,
            minWidth = 120,
            maxWidth = 1000,
            screenWidth = 1220,
            screenHeight = 2712,
        )
        assertEquals(VirtualDisplayWindowFrame(0, 0, 500, 1000), frame)
    }

    @Test
    fun clampsResizeToMinimumAndKeepsSizeInsideScreen() {
        val frame = VirtualDisplayPreviewGeometry.resize(
            start = VirtualDisplayWindowFrame(100, 200, 400, 800),
            corner = VirtualDisplayResizeCorner.BOTTOM_END,
            deltaX = -1000f,
            deltaY = 0f,
            aspectRatio = 2f,
            minWidth = 120,
            maxWidth = 1000,
            screenWidth = 1220,
            screenHeight = 2712,
        )
        assertEquals(VirtualDisplayWindowFrame(100, 200, 120, 240), frame)
    }

    @Test
    fun snapsOnlyWhenCloseToHorizontalEdges() {
        val frame = VirtualDisplayWindowFrame(20, 300, 400, 800)
        assertEquals(
            0,
            VirtualDisplayPreviewGeometry.horizontalSnapTarget(frame, 1220, 32),
        )
        assertEquals(
            820,
            VirtualDisplayPreviewGeometry.horizontalSnapTarget(
                frame.copy(x = 810),
                1220,
                32,
            ),
        )
        assertNull(
            VirtualDisplayPreviewGeometry.horizontalSnapTarget(frame.copy(x = 400), 1220, 32),
        )
    }
}
