package com.amitia.amitia_app.nativeprovider.interaction

import android.content.Context
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.view.accessibility.AccessibilityNodeInfo
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityServiceRegistry
import java.util.concurrent.atomic.AtomicLong

internal class InteractionNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val gestureGeneration = AtomicLong(0L)

    override val operations: Set<String> = setOf(
        OP_STATUS,
        OP_CLICK,
        OP_LONG_CLICK,
        OP_INPUT_TEXT,
        OP_CLEAR_TEXT,
        OP_SCROLL,
        OP_SWIPE,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_CLICK -> handleClick(request)
            OP_LONG_CLICK -> handleLongClick(request)
            OP_INPUT_TEXT -> handleInputText(request)
            OP_CLEAR_TEXT -> handleClearText(request)
            OP_SCROLL -> handleScroll(request)
            OP_SWIPE -> handleSwipe(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "connected" to (service != null),
                "generation" to gestureGeneration.get(),
            ),
        )
    }

    private fun handleClick(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val x = (request.payload["x"] as? Number)?.toInt() ?: -1
        val y = (request.payload["y"] as? Number)?.toInt() ?: -1
        val durationMs = (request.payload["durationMs"] as? Number)?.toLong() ?: 50L

        if (x < 0 || y < 0) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_INVALID_COORDINATES",
                    message = "invalid click coordinates: ($x, $y)",
                ),
            )
        }

        return try {
            val path = Path().apply { moveTo(x.toFloat(), y.toFloat()) }
            val stroke = GestureDescription.StrokeDescription(path, 0, durationMs.coerceAtLeast(1))
            val gesture = GestureDescription.Builder().addStroke(stroke).build()
            val result = service.dispatchGesture(gesture, null, null)
            gestureGeneration.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "performed" to result,
                    "action" to "click",
                    "generation" to gestureGeneration.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_GESTURE_FAILED",
                    message = "click gesture failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleLongClick(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val x = (request.payload["x"] as? Number)?.toInt() ?: -1
        val y = (request.payload["y"] as? Number)?.toInt() ?: -1
        val durationMs = (request.payload["durationMs"] as? Number)?.toLong() ?: 600L

        if (x < 0 || y < 0) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_INVALID_COORDINATES",
                    message = "invalid long click coordinates: ($x, $y)",
                ),
            )
        }

        return try {
            val path = Path().apply { moveTo(x.toFloat(), y.toFloat()) }
            val stroke = GestureDescription.StrokeDescription(path, 0, durationMs.coerceIn(300, 3000))
            val gesture = GestureDescription.Builder().addStroke(stroke).build()
            val result = service.dispatchGesture(gesture, null, null)
            gestureGeneration.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "performed" to result,
                    "action" to "long_click",
                    "generation" to gestureGeneration.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_GESTURE_FAILED",
                    message = "long click gesture failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleInputText(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val text = request.payload["text"] as? String ?: ""

        return try {
            val arguments = android.os.Bundle().apply {
                putCharSequence(
                    AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE,
                    text,
                )
            }
            val focusedNode = service.rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)
            val performed = focusedNode?.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, arguments) ?: false
            gestureGeneration.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = if (performed) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
                result = mapOf(
                    "performed" to performed,
                    "action" to "input_text",
                    "generation" to gestureGeneration.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_INPUT_FAILED",
                    message = "input text failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleClearText(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        return try {
            val arguments = android.os.Bundle().apply {
                putCharSequence(
                    AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE,
                    "",
                )
            }
            val focusedNode = service.rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)
            val performed = focusedNode?.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, arguments) ?: false
            gestureGeneration.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = if (performed) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
                result = mapOf(
                    "performed" to performed,
                    "action" to "clear_text",
                    "generation" to gestureGeneration.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_CLEAR_FAILED",
                    message = "clear text failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleScroll(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val direction = request.payload["direction"] as? String ?: "down"

        val action = when (direction) {
            "forward", "down", "right" -> AccessibilityNodeInfo.ACTION_SCROLL_FORWARD
            "backward", "up", "left" -> AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD
            else -> AccessibilityNodeInfo.ACTION_SCROLL_FORWARD
        }

        return try {
            val focusedNode = service.rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_ACCESSIBILITY)
                ?: service.rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)
            val performed = focusedNode?.performAction(action) ?: false
            gestureGeneration.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = if (performed) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
                result = mapOf(
                    "performed" to performed,
                    "action" to "scroll",
                    "direction" to direction,
                    "generation" to gestureGeneration.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_SCROLL_FAILED",
                    message = "scroll failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleSwipe(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val startX = (request.payload["startX"] as? Number)?.toInt() ?: -1
        val startY = (request.payload["startY"] as? Number)?.toInt() ?: -1
        val endX = (request.payload["endX"] as? Number)?.toInt() ?: -1
        val endY = (request.payload["endY"] as? Number)?.toInt() ?: -1
        val durationMs = (request.payload["durationMs"] as? Number)?.toLong() ?: 300L

        if (startX < 0 || startY < 0 || endX < 0 || endY < 0) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_INVALID_COORDINATES",
                    message = "invalid swipe coordinates",
                ),
            )
        }

        return try {
            val path = Path().apply {
                moveTo(startX.toFloat(), startY.toFloat())
                lineTo(endX.toFloat(), endY.toFloat())
            }
            val stroke = GestureDescription.StrokeDescription(path, 0, durationMs.coerceAtLeast(1))
            val gesture = GestureDescription.Builder().addStroke(stroke).build()
            val result = service.dispatchGesture(gesture, null, null)
            gestureGeneration.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = if (result) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
                result = mapOf(
                    "performed" to result,
                    "action" to "swipe",
                    "generation" to gestureGeneration.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_GESTURE_FAILED",
                    message = "swipe gesture failed: ${e.message}",
                ),
            )
        }
    }

    private fun accessibilityNotConnected(requestId: String): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "INTERACTION_ACCESSIBILITY_NOT_CONNECTED",
                message = "accessibility service not connected",
                domainCode = "ACCESSIBILITY_NOT_CONNECTED",
            ),
        )
    }

    private fun unsupportedOperation(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                message = "unknown interaction operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_STATUS = "interaction.status"
        const val OP_CLICK = "interaction.click"
        const val OP_LONG_CLICK = "interaction.long_click"
        const val OP_INPUT_TEXT = "interaction.input_text"
        const val OP_CLEAR_TEXT = "interaction.clear_text"
        const val OP_SCROLL = "interaction.scroll"
        const val OP_SWIPE = "interaction.swipe"
    }
}
