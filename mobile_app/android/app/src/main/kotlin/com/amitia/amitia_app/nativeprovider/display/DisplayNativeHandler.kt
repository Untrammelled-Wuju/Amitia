package com.amitia.amitia_app.nativeprovider.display

import android.content.Context
import android.hardware.display.DisplayManager
import android.view.Display
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.util.concurrent.atomic.AtomicLong

internal class DisplayNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val generation = AtomicLong(0L)

    override fun supports(operation: String): Boolean {
        return operation == OP_LIST || operation == OP_GET || operation == OP_PRIMARY
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_LIST -> handleList(request)
            OP_GET -> handleGet(request)
            OP_PRIMARY -> handlePrimary(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleList(request: NativeBridgeRequest): NativeBridgeResponse {
        val displays = readDisplays()
        generation.incrementAndGet()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "displays" to displays.map { displayToMap(it) },
                "primaryDisplayId" to displays.firstOrNull { it.isPrimary }?.displayId,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleGet(request: NativeBridgeRequest): NativeBridgeResponse {
        val targetId = (request.payload["displayId"] as? Number)?.toInt()
        val displays = readDisplays()
        val target = if (targetId != null) displays.firstOrNull { it.displayId == targetId } else displays.firstOrNull { it.isPrimary }

        return if (target != null) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = displayToMap(target),
            )
        } else {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "DISPLAY_NOT_FOUND",
                    message = "display not found: $targetId",
                ),
            )
        }
    }

    private fun handlePrimary(request: NativeBridgeRequest): NativeBridgeResponse {
        val displays = readDisplays()
        val primary = displays.firstOrNull { it.isPrimary } ?: displays.firstOrNull()
        return if (primary != null) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = displayToMap(primary),
            )
        } else {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "DISPLAY_NOT_FOUND",
                    message = "no primary display found",
                ),
            )
        }
    }

    private fun readDisplays(): List<DisplayInfo> {
        return try {
            val displayManager = context.getSystemService(Context.DISPLAY_SERVICE) as? DisplayManager
                ?: return emptyList()
            displayManager.displays?.map { display ->
                DisplayInfo(
                    displayId = display.displayId,
                    name = display.name ?: "Display ${display.displayId}",
                    width = display.width,
                    height = display.height,
                    densityDpi = display.densityDpi,
                    rotation = display.rotation,
                    refreshRate = display.refreshRate,
                    isPrimary = display.displayId == Display.DEFAULT_DISPLAY,
                    state = if (display.state != Display.STATE_OFF) "on" else "off",
                )
            } ?: emptyList()
        } catch (_: Exception) {
            emptyList()
        }
    }

    private fun displayToMap(display: DisplayInfo): Map<String, Any?> {
        return mapOf(
            "displayId" to display.displayId,
            "name" to display.name,
            "width" to display.width,
            "height" to display.height,
            "densityDpi" to display.densityDpi,
            "rotation" to display.rotation,
            "refreshRate" to display.refreshRate,
            "isPrimary" to display.isPrimary,
            "state" to display.state,
        )
    }

    private fun unsupportedOperation(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                message = "unknown display operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_LIST = "display.list"
        const val OP_GET = "display.get"
        const val OP_PRIMARY = "display.primary"
    }
}
