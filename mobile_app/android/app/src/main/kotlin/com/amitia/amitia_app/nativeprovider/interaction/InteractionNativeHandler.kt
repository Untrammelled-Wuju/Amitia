package com.amitia.amitia_app.nativeprovider.interaction

import android.content.Context
import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.os.Handler
import android.os.Looper
import android.view.accessibility.AccessibilityNodeInfo
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityProviderClient
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityServiceRegistry
import com.amitia.amitia_app.nativeprovider.devicecontrol.DeviceInteractionAvailability
import com.amitia.amitia_app.nativeprovider.devicecontrol.DeviceInteractionState
import com.amitia.amitia_app.nativeprovider.devicecontrol.DeviceInteractionStateReader
import com.amitia.amitia_app.nativeprovider.uitree.AccessibilityNodeReferenceRegistry
import org.json.JSONArray
import org.json.JSONObject
import java.util.concurrent.atomic.AtomicLong
import kotlin.coroutines.resume
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withTimeoutOrNull

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
        OP_GLOBAL_ACTION,
        OP_GESTURE,
        OP_SCREENSHOT,
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
            OP_GLOBAL_ACTION -> handleGlobalAction(request)
            OP_GESTURE -> handleGesture(request)
            OP_SCREENSHOT -> handleScreenshot(request)
            else -> unsupportedOperation(request)
        }
    }

    private suspend fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        providerStatus()?.let { provider ->
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "connected" to (provider["connected"] == true),
                    "gestureAvailable" to (provider["canPerformGestures"] == true),
                    "generation" to (provider["generation"] ?: gestureGeneration.get()),
                ) + interactionStateReader.read().asMap(),
            )
        }

        val service = AccessibilityServiceRegistry.current()
        val interactionState = interactionStateReader.read()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "connected" to (service != null),
                "gestureAvailable" to (service != null && canPerformGestures(service)),
                "generation" to gestureGeneration.get(),
            ) + interactionState.asMap(),
        )
    }

    private suspend fun handleClick(request: NativeBridgeRequest): NativeBridgeResponse {
        val x = (request.payload["x"] as? Number)?.toInt() ?: -1
        val y = (request.payload["y"] as? Number)?.toInt() ?: -1
        if (x < 0 || y < 0) {
            return invalidCoordinates(request, "click", x, y)
        }
        providerAction(AccessibilityProviderClient.click(context, x, y), request, "click")?.let { return it }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)
        if (!canPerformGestures(service)) return gesturesUnavailable(request.requestId)

        val durationMs = (request.payload["durationMs"] as? Number)?.toLong() ?: 50L

        return try {
            val path = Path().apply { moveTo(x.toFloat(), y.toFloat()) }
            val stroke = GestureDescription.StrokeDescription(path, 0, durationMs.coerceAtLeast(1))
            val gesture = GestureDescription.Builder().addStroke(stroke).build()
            val result = performGesture(service, gesture)
            if (result) gestureGeneration.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = if (result) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
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

    private suspend fun handleLongClick(request: NativeBridgeRequest): NativeBridgeResponse {
        val x = (request.payload["x"] as? Number)?.toInt() ?: -1
        val y = (request.payload["y"] as? Number)?.toInt() ?: -1
        if (x < 0 || y < 0) {
            return invalidCoordinates(request, "long_click", x, y)
        }
        val durationMs = (request.payload["durationMs"] as? Number)?.toLong() ?: 600L
        providerAction(
            AccessibilityProviderClient.longPress(context, x, y, durationMs.coerceIn(300L, 3000L)),
            request,
            "long_click",
        )?.let { return it }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)
        if (!canPerformGestures(service)) return gesturesUnavailable(request.requestId)

        return try {
            val path = Path().apply { moveTo(x.toFloat(), y.toFloat()) }
            val stroke = GestureDescription.StrokeDescription(path, 0, durationMs.coerceIn(300, 3000))
            val gesture = GestureDescription.Builder().addStroke(stroke).build()
            val result = performGesture(service, gesture)
            if (result) gestureGeneration.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = if (result) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
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

    private suspend fun handleInputText(request: NativeBridgeRequest): NativeBridgeResponse {
        val text = request.payload["text"] as? String ?: ""
        val providerNativeRef = (request.payload["nativeRef"] as? String).orEmpty()
        if (AccessibilityProviderClient.isProviderReference(providerNativeRef)) {
            val payload = JSONObject()
                .put("nativeRef", providerNativeRef)
                .put("action", "set_text")
                .put("args", JSONObject().put("text", text))
                .toString()
            providerAction(AccessibilityProviderClient.performNodeAction(context, payload), request, "input_text")?.let { return it }
        }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val targetNode = resolveTargetNode(service, request.payload)

        return try {
            val arguments = android.os.Bundle().apply {
                putCharSequence(
                    AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE,
                    text,
                )
            }
            val node = targetNode
                ?: service.rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)
            node?.performAction(AccessibilityNodeInfo.ACTION_FOCUS)
            val performed = node?.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, arguments) ?: false
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

    private suspend fun handleClearText(request: NativeBridgeRequest): NativeBridgeResponse {
        val providerNativeRef = (request.payload["nativeRef"] as? String).orEmpty()
        if (AccessibilityProviderClient.isProviderReference(providerNativeRef)) {
            val payload = JSONObject()
                .put("nativeRef", providerNativeRef)
                .put("action", "clear_text")
                .put("args", JSONObject())
                .toString()
            providerAction(AccessibilityProviderClient.performNodeAction(context, payload), request, "clear_text")?.let { return it }
        }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)
        val targetNode = resolveTargetNode(service, request.payload)

        return try {
            val arguments = android.os.Bundle().apply {
                putCharSequence(
                    AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE,
                    "",
                )
            }
            val node = targetNode
                ?: service.rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)
            node?.performAction(AccessibilityNodeInfo.ACTION_FOCUS)
            val performed = node?.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, arguments) ?: false
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

    private suspend fun handleScroll(request: NativeBridgeRequest): NativeBridgeResponse {
        val providerNativeRef = (request.payload["nativeRef"] as? String).orEmpty()
        val direction = request.payload["direction"] as? String ?: "down"
        if (AccessibilityProviderClient.isProviderReference(providerNativeRef)) {
            val providerDirection = if (direction in setOf("forward", "down", "right")) "scroll_forward" else "scroll_backward"
            val payload = JSONObject()
                .put("nativeRef", providerNativeRef)
                .put("action", providerDirection)
                .put("args", JSONObject())
                .toString()
            providerAction(AccessibilityProviderClient.performNodeAction(context, payload), request, "scroll")?.let { return it }
        }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)

        val targetNode = resolveTargetNode(service, request.payload)

        val action = when (direction) {
            "forward", "down", "right" -> AccessibilityNodeInfo.ACTION_SCROLL_FORWARD
            "backward", "up", "left" -> AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD
            else -> AccessibilityNodeInfo.ACTION_SCROLL_FORWARD
        }

        return try {
            val node = findScrollableNode(targetNode)
                ?: service.rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_ACCESSIBILITY)
                ?: service.rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)
                ?: findFirstScrollable(service.rootInActiveWindow)
            val performed = node?.performAction(action) ?: false
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

    private suspend fun handleSwipe(request: NativeBridgeRequest): NativeBridgeResponse {
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
        providerAction(
            AccessibilityProviderClient.swipe(context, startX, startY, endX, endY, durationMs.coerceAtLeast(1L)),
            request,
            "swipe",
        )?.let { return it }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)
        if (!canPerformGestures(service)) return gesturesUnavailable(request.requestId)

        return try {
            val path = Path().apply {
                moveTo(startX.toFloat(), startY.toFloat())
                lineTo(endX.toFloat(), endY.toFloat())
            }
            val stroke = GestureDescription.StrokeDescription(path, 0, durationMs.coerceAtLeast(1))
            val gesture = GestureDescription.Builder().addStroke(stroke).build()
            val result = performGesture(service, gesture)
            if (result) gestureGeneration.incrementAndGet()
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

    private fun canPerformGestures(service: AccessibilityService): Boolean =
        service.serviceInfo.capabilities and AccessibilityServiceInfo.CAPABILITY_CAN_PERFORM_GESTURES != 0

    private fun resolveTargetNode(
        service: AccessibilityService,
        payload: Map<String, Any?>,
    ): AccessibilityNodeInfo? {
        val nativeRef = (payload["nativeRef"] as? String)?.trim().orEmpty()
        if (nativeRef.isEmpty()) return null
        return AccessibilityNodeReferenceRegistry.resolve(service, nativeRef)
    }

    private fun findScrollableNode(node: AccessibilityNodeInfo?): AccessibilityNodeInfo? {
        if (node == null) return null
        if (node.isScrollable) return node
        var parent = node.parent
        while (parent != null) {
            if (parent.isScrollable) return parent
            parent = parent.parent
        }
        return findFirstScrollable(node)
    }

    private fun findFirstScrollable(node: AccessibilityNodeInfo?): AccessibilityNodeInfo? {
        if (node == null) return null
        if (node.isScrollable) return node
        for (index in 0 until node.childCount) {
            findFirstScrollable(node.getChild(index))?.let { return it }
        }
        return null
    }

    private fun gesturesUnavailable(requestId: String): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(
            code = "INTERACTION_GESTURES_UNAVAILABLE",
            message = "accessibility gesture capability is not available",
        ),
    )

    private suspend fun performGesture(service: AccessibilityService, gesture: GestureDescription): Boolean =
        withTimeoutOrNull(5000L) {
            suspendCancellableCoroutine { continuation ->
                val accepted = service.dispatchGesture(
                    gesture,
                    object : AccessibilityService.GestureResultCallback() {
                        override fun onCompleted(gestureDescription: GestureDescription) {
                            if (continuation.isActive) continuation.resume(true)
                        }

                        override fun onCancelled(gestureDescription: GestureDescription) {
                            if (continuation.isActive) continuation.resume(false)
                        }
                    },
                    Handler(Looper.getMainLooper()),
                )
                if (!accepted && continuation.isActive) continuation.resume(false)
            }
        } ?: false


    private suspend fun handlePerformNodeAction(request: NativeBridgeRequest): NativeBridgeResponse {
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
        if (AccessibilityProviderClient.isProviderReference(nativeRef)) {
            val payload = JSONObject()
                .put("nativeRef", nativeRef)
                .put("action", action)
                .put("args", JSONObject(request.payload["args"] as? Map<*, *> ?: emptyMap<Any, Any>()))
                .toString()
            providerAction(AccessibilityProviderClient.performNodeAction(context, payload), request, action)?.let { return it }
        }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)
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

        val args = (request.payload["args"] as? Map<*, *>)
            ?.entries
            ?.associate { it.key.toString() to it.value }
            ?: emptyMap()
        val actionResult = performLocalNodeAction(node, action, args)
        gestureGeneration.incrementAndGet()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = if (actionResult.performed) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
            result = mapOf("success" to actionResult.performed, "action" to action, "generation" to gestureGeneration.get()),
            error = if (actionResult.performed) null else NativeBridgeError(
                code = if (actionResult.message.startsWith("unsupported")) {
                    "INTERACTION_ACTION_UNSUPPORTED"
                } else {
                    "INTERACTION_NODE_ACTION_FAILED"
                },
                message = actionResult.message,
            ),
        )
    }

    private fun resolveGlobalAction(name: String): Pair<String, Int>? = when (name.trim().lowercase()) {
        "back" -> "back" to AccessibilityService.GLOBAL_ACTION_BACK
        "home" -> "home" to AccessibilityService.GLOBAL_ACTION_HOME
        "recents" -> "recents" to AccessibilityService.GLOBAL_ACTION_RECENTS
        "notifications" -> "notifications" to AccessibilityService.GLOBAL_ACTION_NOTIFICATIONS
        "quick_settings" -> "quick_settings" to AccessibilityService.GLOBAL_ACTION_QUICK_SETTINGS
        "power_dialog" -> "power_dialog" to AccessibilityService.GLOBAL_ACTION_POWER_DIALOG
        "toggle_split_screen" -> "toggle_split_screen" to AccessibilityService.GLOBAL_ACTION_TOGGLE_SPLIT_SCREEN
        "lock_screen" -> "lock_screen" to AccessibilityService.GLOBAL_ACTION_LOCK_SCREEN
        "take_screenshot" -> "take_screenshot" to AccessibilityService.GLOBAL_ACTION_TAKE_SCREENSHOT
        "keycode_headset_hook" -> "keycode_headset_hook" to AccessibilityService.GLOBAL_ACTION_KEYCODE_HEADSETHOOK
        "accessibility_shortcut" -> "accessibility_shortcut" to AccessibilityService.GLOBAL_ACTION_ACCESSIBILITY_SHORTCUT
        "accessibility_all_apps" -> "accessibility_all_apps" to AccessibilityService.GLOBAL_ACTION_ACCESSIBILITY_ALL_APPS
        "accessibility_button" -> "accessibility_button" to AccessibilityService.GLOBAL_ACTION_ACCESSIBILITY_BUTTON
        "accessibility_button_chooser" -> "accessibility_button_chooser" to AccessibilityService.GLOBAL_ACTION_ACCESSIBILITY_BUTTON_CHOOSER
        "dismiss_notification_shade" -> "dismiss_notification_shade" to AccessibilityService.GLOBAL_ACTION_DISMISS_NOTIFICATION_SHADE
        "dpad_up" -> "dpad_up" to AccessibilityService.GLOBAL_ACTION_DPAD_UP
        "dpad_down" -> "dpad_down" to AccessibilityService.GLOBAL_ACTION_DPAD_DOWN
        "dpad_left" -> "dpad_left" to AccessibilityService.GLOBAL_ACTION_DPAD_LEFT
        "dpad_right" -> "dpad_right" to AccessibilityService.GLOBAL_ACTION_DPAD_RIGHT
        "dpad_center" -> "dpad_center" to AccessibilityService.GLOBAL_ACTION_DPAD_CENTER
        "media_play_pause" -> "media_play_pause" to AccessibilityService.GLOBAL_ACTION_MEDIA_PLAY_PAUSE
        "menu" -> "menu" to AccessibilityService.GLOBAL_ACTION_MENU
        else -> null
    }

    private data class LocalNodeActionResult(
        val performed: Boolean,
        val message: String,
    )

    private fun performLocalNodeAction(
        node: AccessibilityNodeInfo,
        action: String,
        args: Map<String, Any?>,
    ): LocalNodeActionResult {
        val normalized = action.trim().lowercase()
        return try {
            val performed = when (normalized) {
                "click" -> node.performAction(AccessibilityNodeInfo.ACTION_CLICK)
                "long_click" -> node.performAction(AccessibilityNodeInfo.ACTION_LONG_CLICK)
                "focus" -> node.performAction(AccessibilityNodeInfo.ACTION_FOCUS)
                "clear_focus" -> node.performAction(AccessibilityNodeInfo.ACTION_CLEAR_FOCUS)
                "select" -> node.performAction(AccessibilityNodeInfo.ACTION_SELECT)
                "clear_selection" -> node.performAction(AccessibilityNodeInfo.ACTION_CLEAR_SELECTION)
                "set_text" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_SET_TEXT,
                    android.os.Bundle().apply {
                        putCharSequence(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE,
                            args["text"]?.toString().orEmpty(),
                        )
                    },
                )
                "clear_text" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_SET_TEXT,
                    android.os.Bundle().apply {
                        putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, "")
                    },
                )
                "set_selection" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_SET_SELECTION,
                    android.os.Bundle().apply {
                        putInt(AccessibilityNodeInfo.ACTION_ARGUMENT_SELECTION_START_INT, (args["start"] as? Number)?.toInt() ?: 0)
                        putInt(AccessibilityNodeInfo.ACTION_ARGUMENT_SELECTION_END_INT, (args["end"] as? Number)?.toInt() ?: 0)
                    },
                )
                "copy" -> node.performAction(AccessibilityNodeInfo.ACTION_COPY)
                "cut" -> node.performAction(AccessibilityNodeInfo.ACTION_CUT)
                "paste" -> node.performAction(AccessibilityNodeInfo.ACTION_PASTE)
                "scroll_forward" -> node.performAction(AccessibilityNodeInfo.ACTION_SCROLL_FORWARD)
                "scroll_backward" -> node.performAction(AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD)
                "scroll_up" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_UP.id)
                "scroll_down" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_DOWN.id)
                "scroll_left" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_LEFT.id)
                "scroll_right" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_RIGHT.id)
                "scroll_to_position" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_TO_POSITION.id,
                    android.os.Bundle().apply {
                        putInt(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_ROW_INT,
                            (args["row"] as? Number)?.toInt() ?: 0,
                        )
                        putInt(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_COLUMN_INT,
                            (args["column"] as? Number)?.toInt() ?: 0,
                        )
                    },
                )
                "expand" -> node.performAction(AccessibilityNodeInfo.ACTION_EXPAND)
                "collapse" -> node.performAction(AccessibilityNodeInfo.ACTION_COLLAPSE)
                "dismiss" -> node.performAction(AccessibilityNodeInfo.ACTION_DISMISS)
                "show_on_screen" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SHOW_ON_SCREEN.id)
                "context_click" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_CONTEXT_CLICK.id)
                "accessibility_focus" -> node.performAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS)
                "clear_accessibility_focus" -> node.performAction(AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS)
                "show_tooltip" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SHOW_TOOLTIP.id)
                "hide_tooltip" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_HIDE_TOOLTIP.id)
                "ime_enter" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_IME_ENTER.id)
                "press_and_hold" -> node.performAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_PRESS_AND_HOLD.id)
                "next_at_movement_granularity" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_NEXT_AT_MOVEMENT_GRANULARITY,
                    movementBundle(args),
                )
                "previous_at_movement_granularity" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_PREVIOUS_AT_MOVEMENT_GRANULARITY,
                    movementBundle(args),
                )
                "next_html_element" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_NEXT_HTML_ELEMENT,
                    android.os.Bundle().apply {
                        putString(AccessibilityNodeInfo.ACTION_ARGUMENT_HTML_ELEMENT_STRING, args["element"]?.toString())
                    },
                )
                "previous_html_element" -> node.performAction(
                    AccessibilityNodeInfo.ACTION_PREVIOUS_HTML_ELEMENT,
                    android.os.Bundle().apply {
                        putString(AccessibilityNodeInfo.ACTION_ARGUMENT_HTML_ELEMENT_STRING, args["element"]?.toString())
                    },
                )
                "set_progress" -> node.performAction(
                    AccessibilityNodeInfo.AccessibilityAction.ACTION_SET_PROGRESS.id,
                    android.os.Bundle().apply {
                        putFloat(
                            AccessibilityNodeInfo.ACTION_ARGUMENT_PROGRESS_VALUE,
                            (args["value"] as? Number)?.toFloat() ?: 0f,
                        )
                    },
                )
                else -> return LocalNodeActionResult(false, "unsupported node action: $normalized")
            }
            LocalNodeActionResult(performed, if (performed) "" else "accessibility node action returned false")
        } catch (error: Throwable) {
            LocalNodeActionResult(false, error.message ?: error.javaClass.simpleName)
        }
    }

    private fun movementBundle(args: Map<String, Any?>): android.os.Bundle = android.os.Bundle().apply {
        val granularity = when ((args["granularity"] ?: args["movementGranularity"])?.toString()?.lowercase()) {
            "character" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_CHARACTER
            "word" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_WORD
            "line" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_LINE
            "paragraph" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PARAGRAPH
            "page" -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PAGE
            else -> AccessibilityNodeInfo.MOVEMENT_GRANULARITY_CHARACTER
        }
        putInt(AccessibilityNodeInfo.ACTION_ARGUMENT_MOVEMENT_GRANULARITY_INT, granularity)
        putBoolean(
            AccessibilityNodeInfo.ACTION_ARGUMENT_EXTEND_SELECTION_BOOLEAN,
            args["extendSelection"] as? Boolean ?: false,
        )
    }

    private suspend fun handleGlobalAction(request: NativeBridgeRequest): NativeBridgeResponse {
        val actionName = (request.payload["action"] as? String)?.trim().orEmpty()
        val definition = resolveGlobalAction(actionName)
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "INTERACTION_ACTION_UNSUPPORTED",
                    message = "unsupported global action: $actionName",
                ),
            )
        providerAction(
            AccessibilityProviderClient.globalAction(context, definition.second),
            request,
            definition.first,
        )?.let { return it }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)
        val performed = service.performGlobalAction(definition.second)
        gestureGeneration.incrementAndGet()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = if (performed) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
            result = mapOf(
                "performed" to performed,
                "success" to performed,
                "action" to definition.first,
                "generation" to gestureGeneration.get(),
            ),
            error = if (performed) null else NativeBridgeError(
                code = "INTERACTION_ACTION_FAILED",
                message = "accessibility global action returned false",
            ),
        )
    }

    private suspend fun handleGesture(request: NativeBridgeRequest): NativeBridgeResponse {
        val payloadJson = JSONObject(request.payload).toString()
        providerAction(
            AccessibilityProviderClient.gesture(context, payloadJson),
            request,
            "gesture",
        )?.let { return it }

        val service = AccessibilityServiceRegistry.current()
            ?: return accessibilityNotConnected(request.requestId)
        if (!canPerformGestures(service)) return gesturesUnavailable(request.requestId)
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "INTERACTION_GESTURE_FAILED",
                message = "advanced accessibility gesture requires the accessibility provider",
            ),
        )
    }

    private suspend fun handleScreenshot(request: NativeBridgeRequest): NativeBridgeResponse {
        val displayId = (request.payload["displayId"] as? Number)?.toInt() ?: 0
        AccessibilityProviderClient.screenshot(context, displayId)?.let { raw ->
            val result = jsonToMap(JSONObject(raw))
            if (result["success"] == true) {
                return NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = request.requestId,
                    status = NativeBridgeProtocol.STATUS_SUCCESS,
                    result = result,
                )
            }
            if (result["connected"] != false) {
                return NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = request.requestId,
                    status = NativeBridgeProtocol.STATUS_ERROR,
                    error = NativeBridgeError(
                        code = "INTERACTION_SCREENSHOT_FAILED",
                        message = result["message"]?.toString() ?: "accessibility screenshot failed",
                    ),
                )
            }
        }

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "INTERACTION_SCREENSHOT_FAILED",
                message = "accessibility screenshot requires the accessibility provider",
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

    private suspend fun providerStatus(): Map<String, Any?>? {
        val status = AccessibilityProviderClient.status(context) ?: return null
        return jsonToMap(JSONObject(status))
    }

    private suspend fun providerAction(
        rawResult: String?,
        request: NativeBridgeRequest,
        action: String,
    ): NativeBridgeResponse? {
        val result = rawResult?.let { jsonToMap(JSONObject(it)) } ?: return null
        val success = result["success"] == true
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = if (success) NativeBridgeProtocol.STATUS_SUCCESS else NativeBridgeProtocol.STATUS_ERROR,
            result = mapOf(
                "performed" to success,
                "success" to success,
                "action" to action,
                "generation" to gestureGeneration.incrementAndGet(),
            ),
            error = if (success) null else NativeBridgeError(
                code = "INTERACTION_ACTION_FAILED",
                message = result["message"]?.toString() ?: "accessibility provider action failed",
            ),
        )
    }

    private fun invalidCoordinates(
        request: NativeBridgeRequest,
        action: String,
        x: Int,
        y: Int,
    ): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(
            code = "INTERACTION_INVALID_COORDINATES",
            message = "invalid $action coordinates: ($x, $y)",
        ),
    )

    private fun jsonToMap(value: JSONObject): Map<String, Any?> {
        val result = LinkedHashMap<String, Any?>()
        val keys = value.keys()
        while (keys.hasNext()) {
            val key = keys.next()
            result[key] = jsonValue(value.get(key))
        }
        return result
    }

    private fun jsonValue(value: Any?): Any? = when (value) {
        null, JSONObject.NULL -> null
        is JSONObject -> jsonToMap(value)
        is JSONArray -> List(value.length()) { index -> jsonValue(value.get(index)) }
        else -> value
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
        const val OP_GLOBAL_ACTION = "interaction.global_action"
        const val OP_GESTURE = "interaction.gesture"
        const val OP_SCREENSHOT = "interaction.screenshot"
    }
}
