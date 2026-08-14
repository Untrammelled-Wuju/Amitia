import android.accessibilityservice.AccessibilityService
import android.content.Context
import android.graphics.Rect
import android.view.accessibility.AccessibilityNodeInfo
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityServiceRegistry
import java.util.concurrent.atomic.AtomicLong

internal class UITreeNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val generation = AtomicLong(0L)

    override val operations: Set<String> = setOf(
        OP_STATUS,
        OP_SNAPSHOT,
        OP_FIND,
        OP_GET,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_SNAPSHOT -> handleSnapshot(request)
            OP_FIND -> handleFind(request)
            OP_GET -> handleGet(request)
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
                "rootAvailable" to (service?.rootInActiveWindow != null),
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleSnapshot(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
        if (service == null) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "UI_TREE_ACCESSIBILITY_NOT_CONNECTED",
                    message = "accessibility service not connected",
                    domainCode = "ACCESSIBILITY_NOT_CONNECTED",
                ),
            )
        }

        val rootNode = service.rootInActiveWindow
        if (rootNode == null) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "nodes" to emptyList<Map<String, Any?>>(),
                    "windowCount" to 0,
                    "generation" to generation.get(),
                    "accessibilityConnected" to true,
                ),
            )
        }

        generation.incrementAndGet()
        val nodes = mutableListOf<MutableMap<String, Any?>>()
        val idCounter = AtomicLong(0L)
        collectNodes(rootNode, nodes, idCounter, 0, maxDepth = 50)

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "nodes" to nodes,
                "windowCount" to 1,
                "generation" to generation.get(),
                "accessibilityConnected" to true,
            ),
        )
    }

    private fun collectNodes(
        node: AccessibilityNodeInfo?,
        nodes: MutableList<MutableMap<String, Any?>>,
        idCounter: AtomicLong,
        depth: Int,
        maxDepth: Int,
    ) {
        if (node == null || depth > maxDepth) return
        val nodeId = idCounter.incrementAndGet().toString()
        val bounds = Rect().also { node.getBoundsInScreen(it) }
        val nodeInfo = mutableMapOf<String, Any?>(
            "nodeId" to nodeId,
            "className" to node.className?.toString(),
            "packageName" to node.packageName?.toString(),
            "text" to node.text?.toString(),
            "contentDescription" to node.contentDescription?.toString(),
            "boundsInScreen" to listOf(bounds.left, bounds.top, bounds.right, bounds.bottom),
            "clickable" to node.isClickable,
            "scrollable" to node.isScrollable,
            "enabled" to node.isEnabled,
            "focused" to node.isFocused,
        )
        nodes.add(nodeInfo)

        for (i in 0 until node.childCount) {
            val child = node.getChild(i)
            if (child != null) {
                collectNodes(child, nodes, idCounter, depth + 1, maxDepth)
            }
        }
    }

    private fun handleFind(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
        if (service == null) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "UI_TREE_ACCESSIBILITY_NOT_CONNECTED",
                    message = "accessibility service not connected",
                    domainCode = "ACCESSIBILITY_NOT_CONNECTED",
                ),
            )
        }

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "nodes" to emptyList<Map<String, Any?>>(),
                "generation" to generation.get(),
                "accessibilityConnected" to true,
            ),
        )
    }

    private fun handleGet(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
        if (service == null) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "UI_TREE_ACCESSIBILITY_NOT_CONNECTED",
                    message = "accessibility service not connected",
                    domainCode = "ACCESSIBILITY_NOT_CONNECTED",
                ),
            )
        }

        val targetId = request.payload["nodeId"] as? String
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "node" to null,
                "generation" to generation.get(),
                "accessibilityConnected" to true,
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
                message = "unknown ui_tree operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_STATUS = "ui_tree.status"
        const val OP_SNAPSHOT = "ui_tree.snapshot"
        const val OP_FIND = "ui_tree.find"
        const val OP_GET = "ui_tree.get"
    }
}
