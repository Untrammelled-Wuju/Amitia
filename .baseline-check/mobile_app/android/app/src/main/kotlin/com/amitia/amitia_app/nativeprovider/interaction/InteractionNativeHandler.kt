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
import com.amitia.amitia_app.nativeprovider.devicecontrol.DeviceInteractionAvailability
import com.amitia.amitia_app.nativeprovider.devicecontrol.DeviceInteractionState
import com.amitia.amitia_app.nativeprovider.devicecontrol.DeviceInteractionStateReader
import com.amitia.amitia_app.nativeprovider.uitree.AccessibilityNodeReferenceRegistry
import java.util.concurrent.atomic.AtomicLong

internal class InteractionNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val gestureGeneration = AtomicLong(0L)
    private val interactionStateReader = DeviceInteractionStateReader(context)

    override val operations: Set<String> = setOf(
        OP_STATUS,
        OP_CLICK,
        OP_LONG_CLICK,
        OP_INPUT_TEXT,
        OP_CLEAR_TEXT,
        OP_SCROLL,
        OP_SWIPE,
        OP_PERFORM_NODE_ACTION,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        if (request.operation != OP_STATUS) {
            val interactionState = interactionStateReader.read()
            if (interactionState.availability != DeviceInteractionAvailability.AVAILABLE) {
                return interactionUnavailable(request, interactionState)
            }
        }
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_CLICK -> handleClick(request)
            OP_LONG_CLICK -> handleLongClick(request)
            OP_INPUT_TEXT -> handleInputText(request)
            OP_CLEAR_TEXT -> handleClearText(request)
            OP_SCROLL -> handleScroll(request)
            OP_SWIPE -> handleSwipe(request)
            OP_PERFORM_NODE_ACTION -> handlePerformNodeAction(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
        val interactionState = interactionStateReader.read()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "connected" to (service != null),
                "generation" to gestureGeneration.get(),
            ) + interactionState.asMap(),
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


    private fun handlePerformNodeAction(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)
        val nativeRef = (request.payload["nativeRef"] as? String)?.trim().orEmpty()
        val action = (request.payload["action"] as? String)?.trim().orEmpty()
        if (nativeRef.isEmpty() || action.isEmpty()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_INVALID_NODE_ACTION",
                    message = "nativeRef and action are required",
                ),
            )
        }
        val node = AccessibilityNodeReferenceRegistry.resolve(service, nativeRef)
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_NODE_STALE",
                    message = "node reference is stale or expired",
                ),
            )

        val args = request.payload["args"] as? Map<*, *>
        val performed = try {
            when (action) {
                "click" -> node.performAction(AccessibilityNodeInfo.ACTION_CLICK)
                "long_click" -> node.performAction(AccessibilityNodeInfo.ACTION_LONG_CLICK)
                "set_text" -> {
                    val text = args?.get("text")?.toString().orEmpty()
                    val bundle = android.os.Bundle().apply {
                        putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, text)
                    }
                    node.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, bundle)
                }
                "clear_text" -> {
                    val bundle = android.os.Bundle().apply {
                        putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, "")
                    }
                    node.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, bundle)
                }
                "scroll_forward" -> node.performAction(AccessibilityNodeInfo.ACTION_SCROLL_FORWARD)
                "scroll_backward" -> node.performAction(AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD)
                "focus" -> node.performAction(AccessibilityNodeInfo.ACTION_FOCUS)
                "select" -> node.performAction(AccessibilityNodeInfo.ACTION_SELECT)
                else -> return NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = request.requestId,
                    status = NativeBridgeProtocol.STATUS_ERROR,
                    error = NativeBridgeError(
                        code = "INTERACTION_ACTION_UNSUPPORTED",
                        message = "unsupported node action: $action",
                    ),
                )
            }
        } catch (t: Throwable) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_NODE_ACTION_FAILED",
                    message = t.message ?: t::class.java.simpleName,
                ),
            )
        }
        gestureGeneration.incrementAndGet()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = if (performed) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
            result = mapOf("success" to performed, "action" to action, "generation" to gestureGeneration.get()),
            error = if (performed) null else NativeBridgeError(
                code = "INTERACTION_NODE_ACTION_FAILED",
                message = "accessibility node action returned false",
            ),
        )
    }

    private fun interactionUnavailable(
        request: NativeBridgeRequest,
        state: DeviceInteractionState,
    ): NativeBridgeResponse {
        val (code, message) = when (state.availability) {
            DeviceInteractionAvailability.WAITING_UNLOCK ->
                "DEVICE_WAITING_UNLOCK" to "device is locked; user must unlock before UI automation can continue"
            DeviceInteractionAvailability.WAITING_SCREEN ->
                "DEVICE_WAITING_SCREEN" to "device screen is not interactive; UI automation is waiting for the screen to become available"
            DeviceInteractionAvailability.BLOCKED ->
                "DEVICE_BACKGROUND_RESTRICTED" to "Android background restrictions or Doze currently block reliable UI automation"
            DeviceInteractionAvailability.AVAILABLE ->
                return NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = request.requestId,
                    status = NativeBridgeProtocol.STATUS_SUCCESS,
                    result = state.asMap(),
                )
        }
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            result = state.asMap(),
            error = NativeBridgeError(
                code = code,
                message = message,
                domainCode = code,
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
        const val OP_STATUS = "interaction.status"
        const val OP_CLICK = "interaction.click"
        const val OP_LONG_CLICK = "interaction.long_click"
        const val OP_INPUT_TEXT = "interaction.input_text"
        const val OP_CLEAR_TEXT = "interaction.clear_text"
        const val OP_SCROLL = "interaction.scroll"
        const val OP_SWIPE = "interaction.swipe"
        const val OP_PERFORM_NODE_ACTION = "interaction.perform_node_action"
    }
}
