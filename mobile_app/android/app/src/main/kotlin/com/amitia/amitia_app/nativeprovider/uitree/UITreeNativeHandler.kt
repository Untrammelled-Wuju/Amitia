package com.amitia.amitia_app.nativeprovider.uitree

import android.content.Context
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

    override fun supports(operation: String): Boolean {
        return operation == OP_SNAPSHOT || operation == OP_QUERY
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_SNAPSHOT -> handleSnapshot(request)
            OP_QUERY -> handleQuery(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleSnapshot(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
        if (service == null) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "UITREE_ACCESSIBILITY_NOT_CONNECTED",
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
                    "nodes" to emptyList<UITreeNode>(),
                    "windowCount" to 0,
                    "generation" to generation.get(),
                    "accessibilityConnected" to true,
                ),
            )
        }

        generation.incrementAndGet()

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "nodes" to emptyList<UITreeNode>(),
                "windowCount" to 1,
                "generation" to generation.get(),
                "accessibilityConnected" to true,
            ),
        )
    }

    private fun handleQuery(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
        if (service == null) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "UITREE_ACCESSIBILITY_NOT_CONNECTED",
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
                "nodes" to emptyList<UITreeNode>(),
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
                message = "unknown uitree operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_SNAPSHOT = "uitree.snapshot"
        const val OP_QUERY = "uitree.query"
    }
}
