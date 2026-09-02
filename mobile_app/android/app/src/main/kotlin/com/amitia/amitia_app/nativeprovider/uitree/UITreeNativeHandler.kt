package com.amitia.amitia_app.nativeprovider.uitree

import android.content.Context
import android.graphics.Rect
import android.os.Build
import android.view.accessibility.AccessibilityNodeInfo
import android.view.accessibility.AccessibilityWindowInfo
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityServiceRegistry
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.security.MessageDigest
import java.util.concurrent.atomic.AtomicLong

/** Window-aware Accessibility UI tree source with short-lived stable node refs. */
internal class UITreeNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val generation = AtomicLong(0L)

    override val operations: Set<String> = setOf(OP_STATUS, OP_SNAPSHOT, OP_FIND, OP_GET)

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse = when (request.operation) {
        OP_STATUS -> handleStatus(request)
        OP_SNAPSHOT -> handleSnapshot(request)
        OP_FIND -> handleFind(request)
        OP_GET -> handleGet(request)
        else -> unsupportedOperation(request)
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
        val windows = service?.windows.orEmpty()
        return success(
            request,
            mapOf(
                "connected" to (service != null),
                "rootAvailable" to (service?.rootInActiveWindow != null || windows.any { it.root != null }),
                "canRetrieveWindowContent" to (service != null),
                "multiWindow" to (windows.size > 1),
                "windowCount" to windows.size,
                "generation" to generation.get(),
                "state" to if (service != null) "ready" else "permission_required",
            ),
        )
    }

    private fun handleSnapshot(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return error(request, "UI_TREE_ACCESSIBILITY_NOT_CONNECTED", "accessibility service not connected", "ACCESSIBILITY_NOT_CONNECTED")

        val includeAllWindows = request.payload["includeAllWindows"] as? Boolean ?: true
        val includeInvisible = request.payload["includeInvisible"] as? Boolean ?: false
        val maxDepth = ((request.payload["maxDepth"] as? Number)?.toInt() ?: DEFAULT_MAX_DEPTH).coerceIn(1, HARD_MAX_DEPTH)
        val snapshotGeneration = generation.incrementAndGet()
        AccessibilityNodeReferenceRegistry.beginSnapshot(snapshotGeneration)

        val nativeWindows = service.windows.orEmpty()
        val selectedWindows = if (includeAllWindows) {
            nativeWindows
        } else {
            nativeWindows.filter { it.isActive || it.isFocused }.ifEmpty { nativeWindows.take(1) }
        }

        val nodes = mutableListOf<MutableMap<String, Any?>>()
        val windows = mutableListOf<Map<String, Any?>>()
        var truncated = false

        if (selectedWindows.isNotEmpty()) {
            for (window in selectedWindows.sortedBy { it.layer }) {
                val root = window.root ?: continue
                val windowId = windowId(window.id)
                val rootNodeId = nodeId(snapshotGeneration, window.id, emptyList())
                val bounds = Rect().also { window.getBoundsInScreen(it) }
                windows += mapOf(
                    "windowId" to windowId,
                    "type" to mapWindowType(window.type),
                    "packageName" to root.packageName?.toString().orEmpty(),
                    "title" to window.title?.toString().orEmpty(),
                    "active" to window.isActive,
                    "focused" to window.isFocused,
                    "displayId" to displayId(window),
                    "layer" to window.layer,
                    "left" to bounds.left,
                    "top" to bounds.top,
                    "right" to bounds.right,
                    "bottom" to bounds.bottom,
                    "rootNodeId" to rootNodeId,
                )
                if (!collectNodes(root, nodes, snapshotGeneration, window.id, windowId, emptyList(), null, 0, maxDepth, includeInvisible)) {
                    truncated = true
                    break
                }
            }
        } else {
            // Some OEMs expose rootInActiveWindow while getWindows() is empty.
            val root = service.rootInActiveWindow
            if (root != null) {
                val sentinel = AccessibilityNodeReferenceRegistry.ACTIVE_WINDOW_SENTINEL
                val windowId = windowId(sentinel)
                val bounds = Rect().also { root.getBoundsInScreen(it) }
                val rootNodeId = nodeId(snapshotGeneration, sentinel, emptyList())
                windows += mapOf(
                    "windowId" to windowId,
                    "type" to "application",
                    "packageName" to root.packageName?.toString().orEmpty(),
                    "title" to "",
                    "active" to true,
                    "focused" to true,
                    "displayId" to 0,
                    "layer" to 0,
                    "left" to bounds.left,
                    "top" to bounds.top,
                    "right" to bounds.right,
                    "bottom" to bounds.bottom,
                    "rootNodeId" to rootNodeId,
                )
                truncated = !collectNodes(root, nodes, snapshotGeneration, sentinel, windowId, emptyList(), null, 0, maxDepth, includeInvisible)
            }
        }

        val activeWindowId = windows.firstOrNull { it["active"] == true }?.get("windowId") as? String
        return success(
            request,
            mapOf(
                "nodes" to nodes,
                "windows" to windows,
                "windowCount" to windows.size,
                "activeWindowId" to activeWindowId,
                "generation" to snapshotGeneration,
                "capturedAt" to System.currentTimeMillis(),
                "accessibilityConnected" to true,
                "multiWindow" to (windows.size > 1),
                "stableNodeReference" to true,
                "truncated" to truncated,
            ),
        )
    }

    /** Returns false when node/depth limits require truncation. */
    private fun collectNodes(
        node: AccessibilityNodeInfo,
        nodes: MutableList<MutableMap<String, Any?>>,
        snapshotGeneration: Long,
        nativeWindowId: Int,
        windowId: String,
        childPath: List<Int>,
        parentId: String?,
        depth: Int,
        maxDepth: Int,
        includeInvisible: Boolean,
    ): Boolean {
        if (depth > maxDepth || nodes.size >= MAX_NODES) return false
        if (!includeInvisible && !node.isVisibleToUser) return true

        val id = nodeId(snapshotGeneration, nativeWindowId, childPath)
        val bounds = Rect().also { node.getBoundsInScreen(it) }
        val actions = node.actionList.mapNotNull { actionName(it.id) }.distinct()
        val item = mutableMapOf<String, Any?>(
            "nodeId" to id,
            "parentId" to parentId,
            "windowId" to windowId,
            "className" to node.className?.toString(),
            "packageName" to node.packageName?.toString(),
            "text" to node.text?.toString(),
            "contentDescription" to node.contentDescription?.toString(),
            "resourceId" to node.viewIdResourceName,
            "left" to bounds.left,
            "top" to bounds.top,
            "right" to bounds.right,
            "bottom" to bounds.bottom,
            "visibleToUser" to node.isVisibleToUser,
            "clickable" to node.isClickable,
            "longClickable" to node.isLongClickable,
            "scrollable" to node.isScrollable,
            "enabled" to node.isEnabled,
            "focusable" to node.isFocusable,
            "focused" to node.isFocused,
            "selected" to node.isSelected,
            "checked" to node.isChecked,
            "checkable" to node.isCheckable,
            "editable" to node.isEditable,
            "password" to node.isPassword,
            "actions" to actions,
            "depth" to depth,
            "sourceRef" to AccessibilityNodeReferenceRegistry.build(snapshotGeneration, nativeWindowId, childPath),
        )
        nodes += item

        if (depth == maxDepth && node.childCount > 0) return false
        for (index in 0 until node.childCount) {
            val child = node.getChild(index) ?: continue
            if (!collectNodes(child, nodes, snapshotGeneration, nativeWindowId, windowId, childPath + index, id, depth + 1, maxDepth, includeInvisible)) {
                return false
            }
        }
        return true
    }

    private fun handleFind(request: NativeBridgeRequest): NativeBridgeResponse =
        success(request, mapOf("nodes" to emptyList<Map<String, Any?>>(), "generation" to generation.get()))

    private fun handleGet(request: NativeBridgeRequest): NativeBridgeResponse =
        success(request, mapOf("node" to null, "generation" to generation.get()))

    private fun displayId(window: AccessibilityWindowInfo): Int =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) window.displayId else 0

    private fun mapWindowType(type: Int): String = when (type) {
        AccessibilityWindowInfo.TYPE_APPLICATION -> "application"
        AccessibilityWindowInfo.TYPE_INPUT_METHOD -> "input_method"
        AccessibilityWindowInfo.TYPE_SYSTEM -> "system"
        AccessibilityWindowInfo.TYPE_ACCESSIBILITY_OVERLAY -> "accessibility_overlay"
        else -> "unknown"
    }

    private fun actionName(action: Int): String? = when (action) {
        AccessibilityNodeInfo.ACTION_CLICK -> "ACTION_CLICK"
        AccessibilityNodeInfo.ACTION_LONG_CLICK -> "ACTION_LONG_CLICK"
        AccessibilityNodeInfo.ACTION_SCROLL_FORWARD -> "ACTION_SCROLL_FORWARD"
        AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD -> "ACTION_SCROLL_BACKWARD"
        AccessibilityNodeInfo.ACTION_SET_TEXT -> "ACTION_SET_TEXT"
        AccessibilityNodeInfo.ACTION_FOCUS -> "ACTION_FOCUS"
        AccessibilityNodeInfo.ACTION_CLEAR_FOCUS -> "ACTION_CLEAR_FOCUS"
        AccessibilityNodeInfo.ACTION_SELECT -> "ACTION_SELECT"
        else -> null
    }

    private fun windowId(nativeWindowId: Int): String = "acc-window-$nativeWindowId"

    private fun nodeId(generation: Long, nativeWindowId: Int, childPath: List<Int>): String {
        val semantic = "$generation:$nativeWindowId:${childPath.joinToString(".")}".toByteArray()
        val digest = MessageDigest.getInstance("SHA-256").digest(semantic)
        return "acc_" + digest.take(12).joinToString("") { "%02x".format(it) }
    }

    private fun success(request: NativeBridgeRequest, result: Map<String, Any?>) = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_SUCCESS,
        result = result,
    )

    private fun error(request: NativeBridgeRequest, code: String, message: String, domainCode: String? = null) = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(code = code, message = message, domainCode = domainCode),
    )

    private fun unsupportedOperation(request: NativeBridgeRequest) = error(
        request,
        NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
        "unknown ui_tree operation: ${request.operation}",
    )

    companion object {
        const val OP_STATUS = "ui_tree.status"
        const val OP_SNAPSHOT = "ui_tree.snapshot"
        const val OP_FIND = "ui_tree.find"
        const val OP_GET = "ui_tree.get"
        private const val DEFAULT_MAX_DEPTH = 50
        private const val HARD_MAX_DEPTH = 100
        private const val MAX_NODES = 10_000
    }
}
