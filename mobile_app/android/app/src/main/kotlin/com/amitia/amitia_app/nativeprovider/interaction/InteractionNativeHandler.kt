package com.amitia.amitia_app.nativeprovider.interaction

import android.content.Context
import android.accessibilityservice.GestureDescription
import android.graphics.Path
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

    override fun supports(operation: String): Boolean {
        return operation == OP_TAP ||
            operation == OP_SWIPE ||
            operation == OP_INPUT_TEXT ||
            operation == OP_CLEAR_TEXT ||
            operation == OP_SCROLL ||
            operation == OP_ACTION
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_TAP -> handleTap(request)
            OP_SWIPE -> handleSwipe(request)
            OP_INPUT_TEXT -> handleInputText(request)
            OP_CLEAR_TEXT -> handleClearText(request)
            OP_SCROLL -> handleScroll(request)
            OP_ACTION -> handleAction(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleTap(request: NativeBridgeRequest): NativeBridgeResponse {
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
                    message = "invalid tap coordinates: ($x, $y)",
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
                    "action" to "tap",
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
                    message = "tap gesture failed: ${e.message}",
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
                status = NativeBridgeProtocol.STATUS_SUCCESS,
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

    private fun handleInputText(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val text = request.payload["text"] as? String ?: ""
        val clearFirst = request.payload["clearFirst"] as? Boolean ?: false

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "INTERACTION_NOT_IMPLEMENTED",
                message = "text input requires focused node target",
            ),
        )
    }

    private fun handleClearText(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "INTERACTION_NOT_IMPLEMENTED",
                message = "clear text requires focused node target",
            ),
        )
    }

    private fun handleScroll(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val direction = request.payload["direction"] as? String ?: "down"

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "INTERACTION_NOT_IMPLEMENTED",
                message = "scroll requires target node",
            ),
        )
    }

    private fun handleAction(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val actionId = (request.payload["actionId"] as? Number)?.toInt()
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_INVALID_REQUEST",
                    message = "actionId is required",
                ),
            )

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "INTERACTION_NOT_IMPLEMENTED",
                message = "action dispatch requires target node",
            ),
        )
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
        const val OP_TAP = "interaction.tap"
        const val OP_SWIPE = "interaction.swipe"
        const val OP_INPUT_TEXT = "interaction.input_text"
        const val OP_CLEAR_TEXT = "interaction.clear_text"
        const val OP_SCROLL = "interaction.scroll"
        const val OP_ACTION = "interaction.action"
    }
}
