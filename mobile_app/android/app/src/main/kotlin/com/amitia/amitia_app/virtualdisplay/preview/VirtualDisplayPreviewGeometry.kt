package com.amitia.amitia_app.virtualdisplay.preview

import kotlin.math.abs
import kotlin.math.min
import kotlin.math.roundToInt

internal enum class VirtualDisplayResizeCorner {
    TOP_START,
    TOP_END,
    BOTTOM_START,
    BOTTOM_END,
}

internal data class VirtualDisplayWindowFrame(
    val x: Int,
    val y: Int,
    val width: Int,
    val height: Int,
)

internal object VirtualDisplayPreviewGeometry {
    fun detectCorner(
        touchX: Float,
        touchY: Float,
        width: Int,
        height: Int,
        radius: Float,
    ): VirtualDisplayResizeCorner? {
        if (width <= 0 || height <= 0 || radius <= 0f) return null
        val inStartColumn = touchX <= radius
        val inEndColumn = touchX >= width - radius
        val inTopRow = touchY <= radius
        val inBottomRow = touchY >= height - radius
        return when {
            inStartColumn && inTopRow -> VirtualDisplayResizeCorner.TOP_START
            inEndColumn && inTopRow -> VirtualDisplayResizeCorner.TOP_END
            inStartColumn && inBottomRow -> VirtualDisplayResizeCorner.BOTTOM_START
            inEndColumn && inBottomRow -> VirtualDisplayResizeCorner.BOTTOM_END
            else -> null
        }
    }

    fun resize(
        start: VirtualDisplayWindowFrame,
        corner: VirtualDisplayResizeCorner,
        deltaX: Float,
        deltaY: Float,
        aspectRatio: Float,
        minWidth: Int,
        maxWidth: Int,
        screenWidth: Int,
        screenHeight: Int,
    ): VirtualDisplayWindowFrame {
        val widthFromDeltaX = if (
            corner == VirtualDisplayResizeCorner.TOP_END ||
            corner == VirtualDisplayResizeCorner.BOTTOM_END
        ) {
            start.width + deltaX
        } else {
            start.width - deltaX
        }
        val widthFromDeltaY = if (
            corner == VirtualDisplayResizeCorner.BOTTOM_END ||
            corner == VirtualDisplayResizeCorner.BOTTOM_START
        ) {
            (start.height + deltaY) / aspectRatio.coerceAtLeast(0.01f)
        } else {
            (start.height - deltaY) / aspectRatio.coerceAtLeast(0.01f)
        }
        val candidateWidth = if (abs(deltaX) >= abs(deltaY)) {
            widthFromDeltaX
        } else {
            widthFromDeltaY
        }
        val width = candidateWidth.roundToInt().coerceIn(minWidth, maxWidth)
        val height = (width * aspectRatio).roundToInt().coerceAtMost(screenHeight)
        val anchorEndX = start.x + start.width
        val anchorEndY = start.y + start.height
        val proposedX = if (
            corner == VirtualDisplayResizeCorner.TOP_START ||
            corner == VirtualDisplayResizeCorner.BOTTOM_START
        ) {
            anchorEndX - width
        } else {
            start.x
        }
        val proposedY = if (
            corner == VirtualDisplayResizeCorner.TOP_START ||
            corner == VirtualDisplayResizeCorner.TOP_END
        ) {
            anchorEndY - height
        } else {
            start.y
        }
        return VirtualDisplayWindowFrame(
            x = proposedX.coerceIn(0, (screenWidth - width).coerceAtLeast(0)),
            y = proposedY.coerceIn(0, (screenHeight - height).coerceAtLeast(0)),
            width = width,
            height = height,
        )
    }

    fun horizontalSnapTarget(
        frame: VirtualDisplayWindowFrame,
        screenWidth: Int,
        threshold: Int,
    ): Int? {
        val maxX = (screenWidth - frame.width).coerceAtLeast(0)
        return when {
            frame.x <= threshold -> 0
            maxX - frame.x <= threshold -> maxX
            else -> null
        }
    }

}
