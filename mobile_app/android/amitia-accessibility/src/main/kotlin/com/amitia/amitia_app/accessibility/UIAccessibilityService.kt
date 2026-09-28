package com.amitia.amitia_app.accessibility

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.graphics.Rect
import android.os.Handler
import android.os.Looper
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import android.view.accessibility.AccessibilityWindowInfo
import org.json.JSONArray
import org.json.JSONObject
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicLong

class UIAccessibilityService : AccessibilityService() {

    private val mainHandler = Handler(Looper.getMainLooper())
    private val generation = AtomicLong(0L)
    private val references = LinkedHashMap<String, NodeReference>()
    private var currentGeneration = 0L
    private var lastPackageName = ""
    private var lastActivityName = ""

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        if (event == null) return
        if (event.packageName != null) lastPackageName = event.packageName.toString()
        if (event.className != null) lastActivityName = event.className.toString()
    }

    override fun onInterrupt() {
    }

    override fun onDestroy() {
        if (instance === this) instance = null
        references.clear()
        super.onDestroy()
    }

    fun statusJson(): String {
        return runOnMain {
            val info = serviceInfo
            val capabilities = info?.capabilities ?: 0
            JSONObject()
                .put("connected", true)
                .put("enabledInSettings", true)
                .put("canRetrieveWindowContent", capabilities and AccessibilityServiceInfo.CAPABILITY_CAN_RETRIEVE_WINDOW_CONTENT != 0)
                .put("canRetrieveInteractiveWindows", info?.flags?.and(AccessibilityServiceInfo.FLAG_RETRIEVE_INTERACTIVE_WINDOWS) != 0)
                .put("canPerformGestures", capabilities and AccessibilityServiceInfo.CAPABILITY_CAN_PERFORM_GESTURES != 0)
                .put("canTakeScreenshot", capabilities and AccessibilityServiceInfo.CAPABILITY_CAN_TAKE_SCREENSHOT != 0)
                .put("includeNotImportantViews", info?.flags?.and(AccessibilityServiceInfo.FLAG_INCLUDE_NOT_IMPORTANT_VIEWS) != 0)
                .put("enhancedWebAccessibility", info?.flags?.and(AccessibilityServiceInfo.FLAG_REQUEST_ENHANCED_WEB_ACCESSIBILITY) != 0)
                .put("packageName", lastPackageName)
                .put("activityName", lastActivityName)
                .toString()
        }
    }

    fun snapshotJson(payloadJson: String): String = runOnMain {
        val payload = if (payloadJson.isBlank()) JSONObject() else JSONObject(payloadJson)
        val includeAllWindows = payload.optBoolean("includeAllWindows", true)
        val includeInvisible = payload.optBoolean("includeInvisible", false)
        val maxDepth = payload.optInt("maxDepth", DEFAULT_MAX_DEPTH).coerceIn(1, HARD_MAX_DEPTH)
        val snapshotGeneration = generation.incrementAndGet()
        currentGeneration = snapshotGeneration
        references.clear()

        val allWindows = windows.orEmpty()
        val selectedWindows = if (includeAllWindows) {
            allWindows
        } else {
            allWindows.filter { it.isActive || it.isFocused }.ifEmpty { allWindows.take(1) }
        }

        val nodes = JSONArray()
        val outputWindows = JSONArray()
        var truncated = false
        if (selectedWindows.isNotEmpty()) {
            for (window in selectedWindows.sortedBy { it.layer }) {
                val root = window.root ?: continue
                val windowId = window.windowId()
                val rootNodeId = nodeId(snapshotGeneration, window.id, emptyList())
                val bounds = Rect().also { window.getBoundsInScreen(it) }
                outputWindows.put(
                    JSONObject()
                        .put("windowId", windowId)
                        .put("type", windowType(window.type))
                        .put("packageName", root.packageName?.toString().orEmpty())
                        .put("title", window.title?.toString().orEmpty())
                        .put("active", window.isActive)
                        .put("focused", window.isFocused)
                        .put("displayId", displayId(window))
                        .put("layer", window.layer)
                        .put("left", bounds.left)
                        .put("top", bounds.top)
                        .put("right", bounds.right)
                        .put("bottom", bounds.bottom)
                        .put("rootNodeId", rootNodeId),
                )
                if (!appendNode(root, nodes, snapshotGeneration, window.id, windowId, emptyList(), null, 0, maxDepth, includeInvisible)) {
                    truncated = true
                    break
                }
            }
        } else {
            val root = rootInActiveWindow
            if (root != null) {
                val sentinel = ACTIVE_WINDOW_SENTINEL
                val windowId = windowId(sentinel)
                val bounds = Rect().also { root.getBoundsInScreen(it) }
                val rootNodeId = nodeId(snapshotGeneration, sentinel, emptyList())
                outputWindows.put(
                    JSONObject()
                        .put("windowId", windowId)
                        .put("type", "application")
                        .put("packageName", root.packageName?.toString().orEmpty())
                        .put("title", "")
                        .put("active", true)
                        .put("focused", true)
                        .put("displayId", 0)
                        .put("layer", 0)
                        .put("left", bounds.left)
                        .put("top", bounds.top)
                        .put("right", bounds.right)
                        .put("bottom", bounds.bottom)
                        .put("rootNodeId", rootNodeId),
                )
                truncated = !appendNode(root, nodes, snapshotGeneration, sentinel, windowId, emptyList(), null, 0, maxDepth, includeInvisible)
            }
        }

        var activeWindowId = ""
        for (index in 0 until outputWindows.length()) {
            val window = outputWindows.getJSONObject(index)
            if (window.optBoolean("active", false)) {
                activeWindowId = window.optString("windowId")
                break
            }
        }
        JSONObject()
            .put("nodes", nodes)
            .put("windows", outputWindows)
            .put("windowCount", outputWindows.length())
            .put("activeWindowId", activeWindowId)
            .put("generation", snapshotGeneration)
            .put("capturedAt", System.currentTimeMillis())
            .put("accessibilityConnected", true)
            .put("multiWindow", outputWindows.length() > 1)
            .put("stableNodeReference", true)
            .put("truncated", truncated)
            .toString()
    }

    fun performNodeActionJson(payloadJson: String): String = runOnMain {
        val payload = JSONObject(payloadJson)
        val nativeRef = payload.optString("nativeRef")
        val action = payload.optString("action")
        val args = payload.optJSONObject("args")
        val node = resolveReference(nativeRef)
        if (node == null) return@runOnMain actionResult(false, action, "node reference is stale or expired").toString()
        val argsMap = args?.let { jsonObjectToMap(it) } ?: emptyMap()
        val result = AccessibilityNodeActionExecutor.perform(node, action, argsMap)
        actionResult(result.performed, action, result.message).toString()
    }

    fun performGestureJson(payloadJson: String): String {
        val payload = if (payloadJson.isBlank()) JSONObject() else JSONObject(payloadJson)
        val parsed = AccessibilityGestureFactory.parse(payload)
        val gesture = parsed.gesture
            ?: return actionResult(false, "gesture", parsed.message).toString()
        return actionResult(dispatchGestureAndWait(gesture), "gesture", "").toString()
    }

    fun takeScreenshotJson(displayId: Int): String {
        val result = AccessibilityScreenshotCapture.capture(this, displayId)
        return JSONObject()
            .put("success", result.success)
            .put("imageBase64", result.imageBase64)
            .put("mimeType", result.mimeType)
            .put("width", result.width)
            .put("height", result.height)
            .put("message", result.message)
            .toString()
    }

    fun performClick(x: Int, y: Int): String {
        if (x < 0 || y < 0) return actionResult(false, "click", "invalid coordinates").toString()
        val path = Path().apply { moveTo(x.toFloat(), y.toFloat()) }
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(path, 0L, 50L))
            .build()
        return actionResult(dispatchGestureAndWait(gesture), "click", "").toString()
    }

    fun performLongPress(x: Int, y: Int, durationMs: Long): String {
        if (x < 0 || y < 0) return actionResult(false, "long_click", "invalid coordinates").toString()
        val path = Path().apply { moveTo(x.toFloat(), y.toFloat()) }
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(path, 0L, durationMs.coerceIn(300L, 3000L)))
            .build()
        return actionResult(dispatchGestureAndWait(gesture), "long_click", "").toString()
    }

    fun performSwipe(startX: Int, startY: Int, endX: Int, endY: Int, durationMs: Long): String {
        if (startX < 0 || startY < 0 || endX < 0 || endY < 0) {
            return actionResult(false, "swipe", "invalid coordinates").toString()
        }
        val path = Path().apply {
            moveTo(startX.toFloat(), startY.toFloat())
            lineTo(endX.toFloat(), endY.toFloat())
        }
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(path, 0L, durationMs.coerceAtLeast(1L)))
            .build()
        return actionResult(dispatchGestureAndWait(gesture), "swipe", "").toString()
    }

    fun performGlobalActionJson(actionId: Int): String = runOnMain {
        val performed = super.performGlobalAction(actionId)
        actionResult(performed, "global_action", "").toString()
    }

    private fun appendNode(
        node: AccessibilityNodeInfo,
        nodes: JSONArray,
        snapshotGeneration: Long,
        nativeWindowId: Int,
        windowId: String,
        childPath: List<Int>,
        parentId: String?,
        depth: Int,
        maxDepth: Int,
        includeInvisible: Boolean,
    ): Boolean {
        if (depth > maxDepth || nodes.length() >= MAX_NODES) return false
        if (!includeInvisible && !node.isVisibleToUser) return true
        val id = nodeId(snapshotGeneration, nativeWindowId, childPath)
        val sourceRef = reference(snapshotGeneration, nativeWindowId, childPath)
        references[sourceRef] = NodeReference(snapshotGeneration, nativeWindowId, childPath)
        val bounds = Rect().also { node.getBoundsInScreen(it) }
        val actions = JSONArray()
        node.actionList.forEach { action ->
            actionName(action.id)?.let { actions.put(it) }
        }
        nodes.put(
            JSONObject()
                .put("nodeId", id)
                .put("parentId", parentId ?: JSONObject.NULL)
                .put("windowId", windowId)
                .put("className", node.className?.toString() ?: JSONObject.NULL)
                .put("packageName", node.packageName?.toString() ?: JSONObject.NULL)
                .put("text", node.text?.toString() ?: JSONObject.NULL)
                .put("contentDescription", node.contentDescription?.toString() ?: JSONObject.NULL)
                .put("resourceId", node.viewIdResourceName ?: JSONObject.NULL)
                .put("left", bounds.left)
                .put("top", bounds.top)
                .put("right", bounds.right)
                .put("bottom", bounds.bottom)
                .put("visibleToUser", node.isVisibleToUser)
                .put("clickable", node.isClickable)
                .put("longClickable", node.isLongClickable)
                .put("scrollable", node.isScrollable)
                .put("enabled", node.isEnabled)
                .put("focusable", node.isFocusable)
                .put("focused", node.isFocused)
                .put("selected", node.isSelected)
                .put("checked", node.isChecked)
                .put("checkable", node.isCheckable)
                .put("editable", node.isEditable)
                .put("password", node.isPassword)
                .put("actions", actions)
                .put("depth", depth)
                .put("sourceRef", sourceRef),
        )
        if (depth == maxDepth && node.childCount > 0) return false
        for (index in 0 until node.childCount) {
            val child = node.getChild(index) ?: continue
            if (!appendNode(child, nodes, snapshotGeneration, nativeWindowId, windowId, childPath + index, id, depth + 1, maxDepth, includeInvisible)) {
                return false
            }
        }
        return true
    }

    private fun resolveReference(value: String): AccessibilityNodeInfo? {
        val parts = value.split(':', limit = 4)
        if (parts.size != 4 || parts[0] != "api") return null
        val refGeneration = parts[1].toLongOrNull() ?: return null
        if (refGeneration != currentGeneration) return null
        val nativeWindowId = parts[2].toIntOrNull() ?: return null
        val path = if (parts[3].isEmpty() || parts[3] == "root") {
            emptyList()
        } else {
            parts[3].split('.').map { it.toIntOrNull() ?: return null }
        }
        var node = if (nativeWindowId == ACTIVE_WINDOW_SENTINEL) {
            rootInActiveWindow ?: return null
        } else {
            windows.orEmpty().firstOrNull { it.id == nativeWindowId }?.root ?: return null
        }
        for (index in path) {
            if (index < 0 || index >= node.childCount) return null
            node = node.getChild(index) ?: return null
        }
        return node
    }

    private fun dispatchGestureAndWait(gesture: GestureDescription): Boolean {
        if (Looper.myLooper() == Looper.getMainLooper()) return false
        val latch = CountDownLatch(1)
        val result = java.util.concurrent.atomic.AtomicBoolean(false)
        val dispatchLatch = CountDownLatch(1)
        val dispatchError = java.util.concurrent.atomic.AtomicReference<Throwable?>()
        mainHandler.post {
            try {
                val accepted = dispatchGesture(
                    gesture,
                    object : GestureResultCallback() {
                        override fun onCompleted(gestureDescription: GestureDescription?) {
                            result.set(true)
                            latch.countDown()
                        }

                        override fun onCancelled(gestureDescription: GestureDescription?) {
                            result.set(false)
                            latch.countDown()
                        }
                    },
                    mainHandler,
                )
                if (!accepted) {
                    result.set(false)
                    latch.countDown()
                }
            } catch (throwable: Throwable) {
                dispatchError.set(throwable)
                result.set(false)
                latch.countDown()
            } finally {
                dispatchLatch.countDown()
            }
        }
        try {
            if (!dispatchLatch.await(5, TimeUnit.SECONDS)) return false
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
            return false
        }
        dispatchError.get()?.let { throw it }
        return try {
            latch.await(5, TimeUnit.SECONDS) && result.get()
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
            false
        }
    }

    private fun actionResult(success: Boolean, action: String, message: String): JSONObject =
        JSONObject()
            .put("success", success)
            .put("action", action)
            .put("message", message)

    private fun <T> runOnMain(block: () -> T): T {
        if (Looper.myLooper() == Looper.getMainLooper()) return block()
        val value = java.util.concurrent.atomic.AtomicReference<T>()
        val error = java.util.concurrent.atomic.AtomicReference<Throwable>()
        val latch = CountDownLatch(1)
        mainHandler.post {
            try {
                value.set(block())
            } catch (throwable: Throwable) {
                error.set(throwable)
            } finally {
                latch.countDown()
            }
        }
        latch.await(5, TimeUnit.SECONDS)
        error.get()?.let { throw it }
        return value.get() ?: throw IllegalStateException("accessibility provider did not respond")
    }

    private fun windowId(nativeWindowId: Int): String = "api-window-$nativeWindowId"

    private fun AccessibilityWindowInfo.windowId(): String = windowId(id)

    private fun nodeId(snapshotGeneration: Long, nativeWindowId: Int, childPath: List<Int>): String =
        "api_node_${snapshotGeneration}_${nativeWindowId}_${if (childPath.isEmpty()) "root" else childPath.joinToString(".")}"

    private fun reference(snapshotGeneration: Long, nativeWindowId: Int, childPath: List<Int>): String =
        "api:$snapshotGeneration:$nativeWindowId:${if (childPath.isEmpty()) "root" else childPath.joinToString(".")}"

    private fun displayId(window: AccessibilityWindowInfo): Int =
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.R) window.displayId else 0

    private fun windowType(type: Int): String = when (type) {
        AccessibilityWindowInfo.TYPE_APPLICATION -> "application"
        AccessibilityWindowInfo.TYPE_INPUT_METHOD -> "input_method"
        AccessibilityWindowInfo.TYPE_SYSTEM -> "system"
        AccessibilityWindowInfo.TYPE_ACCESSIBILITY_OVERLAY -> "accessibility_overlay"
        else -> "unknown"
    }

    private fun actionName(action: Int): String? = AccessibilityNodeActionExecutor.actionName(action)

    private fun jsonObjectToMap(value: JSONObject): Map<String, Any?> {
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
        is JSONObject -> jsonObjectToMap(value)
        is JSONArray -> List(value.length()) { index -> jsonValue(value.get(index)) }
        else -> value
    }

    private data class NodeReference(
        val generation: Long,
        val nativeWindowId: Int,
        val childPath: List<Int>,
    )

    companion object {
        @Volatile
        private var instance: UIAccessibilityService? = null

        fun current(): UIAccessibilityService? = instance

        private const val ACTIVE_WINDOW_SENTINEL = -1
        private const val DEFAULT_MAX_DEPTH = 50
        private const val HARD_MAX_DEPTH = 100
        private const val MAX_NODES = 10000
    }
}
